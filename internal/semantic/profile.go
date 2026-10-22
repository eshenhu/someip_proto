package semantic

import (
	"fmt"
	"net/netip"
	"regexp"
	"sort"
	"strings"

	manifestv1alpha1 "github.com/eshenhu/someip_proto/gen/go/io/github/someip_proto/manifest/v1alpha1"
	optionsv1alpha1 "github.com/eshenhu/someip_proto/gen/go/io/github/someip_proto/options/v1alpha1"
	"github.com/eshenhu/someip_proto/internal/descriptor"
	"google.golang.org/protobuf/reflect/protoreflect"
)

var profileIdentifierPattern = regexp.MustCompile(`^[a-z][a-z0-9._-]{0,127}$`)

func (n *normalizer) normalizeProfile(definition profileDefinition) {
	profile := definition.definition
	location := n.fileOptionsLocation(definition)
	fileOptions := descriptor.FileOptions(definition.file)
	n.validateAnnotatedOrigin(definition.file, "deployment profile")
	n.validateClosedProfile(definition)
	if !profileIdentifierPattern.MatchString(profile.GetProfileName()) {
		n.diagnostics.error(ruleProfile, location, fmt.Sprintf("profile_name %q must be a lowercase ASCII identifier of at most 128 characters", profile.GetProfileName()))
	}
	if profile.GetVersion() == nil {
		n.diagnostics.error(ruleProfile, location, "profile semantic version is required")
	} else {
		validateSemanticVersion(&n.diagnostics, location, profile.GetVersion())
	}
	if profile.GetUdpApplicationPayloadBudgetBytes() == 0 {
		n.diagnostics.error(ruleProfile, location, "udp_application_payload_budget_bytes must be nonzero")
	}

	manifest := &manifestv1alpha1.ProfileManifest{
		ProfileName:                      profile.GetProfileName(),
		Version:                          profile.GetVersion(),
		UdpApplicationPayloadBudgetBytes: profile.GetUdpApplicationPayloadBudgetBytes(),
		Location:                         location.Manifest(),
		DocumentationUris:                canonicalDocumentationURIs(fileOptions.GetDocumentationUris()),
	}
	n.manifest.Profile = manifest

	providerNames := make(map[string]struct{})
	providerPairs := make(map[[2]uint32]struct{})
	serviceLineages := make(map[uint32]string)
	endpointOwners := make(map[uint32]map[string]string)
	for _, provider := range profile.GetProviderInstances() {
		providerManifest := n.normalizeProvider(
			definition,
			provider,
			location,
			providerNames,
			providerPairs,
			serviceLineages,
			endpointOwners,
		)
		manifest.ProviderInstances = append(manifest.ProviderInstances, providerManifest)
	}
	if len(profile.GetProviderInstances()) == 0 {
		n.diagnostics.error(ruleProfile, location, "deployment profile must declare at least one provider instance")
	}
	n.finalizePayloads(definition.file)
	n.updateProviderPayloadSizes(definition, manifest, location)
	n.normalizeProviderTP(profile.GetUdpApplicationPayloadBudgetBytes())
	n.normalizeProviderSD()
	n.normalizeConsumerBindings(definition, manifest, profile)
	sort.Slice(manifest.ProviderInstances, func(i, j int) bool {
		left, right := manifest.ProviderInstances[i].GetInstance(), manifest.ProviderInstances[j].GetInstance()
		if left.GetServiceId() != right.GetServiceId() {
			return left.GetServiceId() < right.GetServiceId()
		}
		if left.GetInstanceId() != right.GetInstanceId() {
			return left.GetInstanceId() < right.GetInstanceId()
		}
		return left.GetProviderName() < right.GetProviderName()
	})

	n.sortManifestContents()
}

func (n *normalizer) validateClosedProfile(definition profileDefinition) {
	seen := map[string]struct{}{string(definition.file.Path()): {}}
	pending := make([]protoreflect.FileDescriptor, 0, definition.file.Imports().Len())
	for index := range definition.file.Imports().Len() {
		pending = append(pending, definition.file.Imports().Get(index).FileDescriptor)
	}
	for len(pending) > 0 {
		last := len(pending) - 1
		file := pending[last]
		pending = pending[:last]
		if file == nil {
			continue
		}
		path := string(file.Path())
		if _, found := seen[path]; found {
			continue
		}
		seen[path] = struct{}{}
		if descriptor.FileOptions(file).GetDeploymentProfile() != nil {
			n.diagnostics.error(ruleProfile, n.fileOptionsLocation(definition), fmt.Sprintf("deployment profile %q imports another deployment profile file %q", definition.definition.GetProfileName(), path))
		}
		for index := range file.Imports().Len() {
			pending = append(pending, file.Imports().Get(index).FileDescriptor)
		}
	}
}

func (n *normalizer) updateProviderPayloadSizes(
	definition profileDefinition,
	profile *manifestv1alpha1.ProfileManifest,
	location descriptor.Location,
) {
	for _, provider := range profile.GetProviderInstances() {
		provider.MaximumApplicationPayloadBytes = 0
		provider.MaximumSomeipMessageBytes = 0
		for _, binding := range provider.GetServiceVersionBindings() {
			name, valid := normalizedFQN(binding.GetBinding().GetService())
			if !valid {
				continue
			}
			service := n.services[name]
			if service == nil {
				continue
			}
			binding.MaximumApplicationPayloadBytes = service.maxPayload
			binding.MaximumSomeipMessageBytes = n.someipMessageSize(service.maxPayload, location, binding.GetBinding().GetService().GetFqn())
			if service.maxPayload > provider.GetMaximumApplicationPayloadBytes() {
				provider.MaximumApplicationPayloadBytes = service.maxPayload
				provider.MaximumSomeipMessageBytes = binding.GetMaximumSomeipMessageBytes()
			}
		}
	}
}

func maximumUDPApplicationPayload(
	service *serviceInfo,
	binding *optionsv1alpha1.ServiceVersionBinding,
	provider *optionsv1alpha1.ProviderInstance,
) (uint64, bool) {
	if service == nil || binding == nil || provider == nil {
		return 0, false
	}
	endpoints := make(map[string]*optionsv1alpha1.Endpoint, len(provider.GetEndpoints()))
	for _, endpoint := range provider.GetEndpoints() {
		endpoints[endpoint.GetName()] = endpoint
	}
	defaultEndpoint := endpoints[provider.GetDefaultEndpointName()]
	if defaultEndpoint == nil {
		return 0, false
	}
	overrides := make(map[uint32]string, len(binding.GetMemberEndpointOverrides()))
	for _, override := range binding.GetMemberEndpointOverrides() {
		overrides[override.GetMemberId()] = override.GetEndpointName()
	}
	maximum := uint64(0)
	usesUDP := false
	for memberID, bounds := range service.memberPayloadBounds {
		endpoint := defaultEndpoint
		if endpointName, found := overrides[memberID]; found && endpoints[endpointName] != nil {
			endpoint = endpoints[endpointName]
		}
		if endpoint.GetTransport() != optionsv1alpha1.TransportProtocol_TRANSPORT_PROTOCOL_UDP {
			continue
		}
		usesUDP = true
		for _, bound := range bounds {
			if bound.size.maximum > maximum {
				maximum = bound.size.maximum
			}
		}
	}
	return maximum, usesUDP
}

func (n *normalizer) normalizeProvider(
	definition profileDefinition,
	provider *optionsv1alpha1.ProviderInstance,
	location descriptor.Location,
	providerNames map[string]struct{},
	providerPairs map[[2]uint32]struct{},
	serviceLineages map[uint32]string,
	endpointOwners map[uint32]map[string]string,
) *manifestv1alpha1.ProviderInstanceManifest {
	provider = canonicalProviderInstance(provider)
	if provider.GetService() != nil || len(provider.GetMemberEndpointOverrides()) > 0 {
		n.diagnostics.error(ruleArtifactCanonical, location, "deprecated provider service or direct member endpoint overrides are not canonical semantic input")
	}
	if !profileIdentifierPattern.MatchString(provider.GetProviderName()) {
		n.diagnostics.error(ruleProfile, location, fmt.Sprintf("provider_name %q must be a lowercase ASCII identifier of at most 128 characters", provider.GetProviderName()))
	}
	if _, found := providerNames[provider.GetProviderName()]; found {
		n.diagnostics.error(ruleProfile, location, fmt.Sprintf("provider_name %q is not unique within the profile", provider.GetProviderName()))
	}
	providerNames[provider.GetProviderName()] = struct{}{}
	validateServiceID(&n.diagnostics, location, provider.GetServiceId())
	validateInstanceID(&n.diagnostics, location, provider.GetInstanceId())
	pair := [2]uint32{provider.GetServiceId(), provider.GetInstanceId()}
	if _, found := providerPairs[pair]; found {
		n.diagnostics.error(ruleProfile, location, fmt.Sprintf("duplicate provider identity service_id=%d instance_id=%d", pair[0], pair[1]))
	}
	providerPairs[pair] = struct{}{}

	lineage := provider.GetServiceLineage().GetId()
	if lineage == "" {
		n.diagnostics.error(rulePolicyLineage, location, "provider identity must declare a nonempty service lineage")
	}
	if existing, found := serviceLineages[provider.GetServiceId()]; found && existing != lineage {
		n.diagnostics.error(ruleProfile, location, fmt.Sprintf("service_id %d maps to both lineages %q and %q", provider.GetServiceId(), existing, lineage))
	}
	serviceLineages[provider.GetServiceId()] = lineage

	endpoints := make(map[string]*optionsv1alpha1.Endpoint)
	for _, endpoint := range provider.GetEndpoints() {
		if !profileIdentifierPattern.MatchString(endpoint.GetName()) {
			n.diagnostics.error(ruleProfile, location, fmt.Sprintf("endpoint name %q must be a lowercase ASCII identifier of at most 128 characters", endpoint.GetName()))
		}
		if _, found := endpoints[endpoint.GetName()]; found {
			n.diagnostics.error(ruleProfile, location, fmt.Sprintf("endpoint name %q is duplicated for provider %q", endpoint.GetName(), provider.GetProviderName()))
		}
		endpoints[endpoint.GetName()] = endpoint
		validateEndpoint(&n.diagnostics, location, endpoint)

		tuple := endpointTuple(endpoint)
		owners := endpointOwners[provider.GetServiceId()]
		if owners == nil {
			owners = make(map[string]string)
			endpointOwners[provider.GetServiceId()] = owners
		}
		if owner, found := owners[tuple]; found && owner != provider.GetProviderName() {
			n.diagnostics.error(ruleProfile, location, fmt.Sprintf("endpoint %s is shared by distinct provider identities of service_id %d", tuple, provider.GetServiceId()))
		}
		owners[tuple] = provider.GetProviderName()
	}
	if len(endpoints) == 0 {
		n.diagnostics.error(ruleProfile, location, fmt.Sprintf("provider %q must declare at least one endpoint", provider.GetProviderName()))
	}
	if _, found := endpoints[provider.GetDefaultEndpointName()]; !found {
		n.diagnostics.error(ruleProfile, location, fmt.Sprintf("default endpoint %q is not declared by provider %q", provider.GetDefaultEndpointName(), provider.GetProviderName()))
	}

	providerManifest := &manifestv1alpha1.ProviderInstanceManifest{
		Instance: provider,
		Location: location.Manifest(),
	}
	bindingKeys := make(map[string]struct{})
	if len(provider.GetServiceVersionBindings()) == 0 {
		n.diagnostics.error(rulePolicyLineage, location, fmt.Sprintf("provider %q must declare at least one service-version binding", provider.GetProviderName()))
	}
	for _, binding := range provider.GetServiceVersionBindings() {
		service := n.resolveService(binding.GetService(), location)
		if service == nil {
			continue
		}
		serviceInfo := n.normalizeService(service)
		if serviceInfo == nil {
			continue
		}
		bindingKey := serviceInfo.options.GetServiceLineage().GetId() + "/" + fmt.Sprint(binding.GetInterfaceMajorVersion())
		if _, found := bindingKeys[bindingKey]; found {
			n.diagnostics.error(ruleProfile, location, fmt.Sprintf("duplicate binding for lineage/major %q", bindingKey))
		}
		bindingKeys[bindingKey] = struct{}{}
		if binding.GetInterfaceMajorVersion() != serviceInfo.options.GetInterfaceMajorVersion() {
			n.diagnostics.error(ruleProfile, location, fmt.Sprintf("binding major version %d does not match service %s major version %d", binding.GetInterfaceMajorVersion(), service.FullName(), serviceInfo.options.GetInterfaceMajorVersion()))
		}
		if serviceInfo.options.GetServiceLineage().GetId() != lineage {
			n.diagnostics.error(rulePolicyLineage, location, fmt.Sprintf("service %s belongs to lineage %q, not provider lineage %q", service.FullName(), serviceInfo.options.GetServiceLineage().GetId(), lineage))
		}
		n.validateMemberEndpointOverrides(serviceInfo, binding, endpoints, location)
		bindingManifest := &manifestv1alpha1.ProviderServiceVersionBindingManifest{
			Binding:                        binding,
			ServiceLineage:                 serviceInfo.options.GetServiceLineage(),
			MaximumApplicationPayloadBytes: 0,
			MaximumSomeipMessageBytes:      0,
			Location:                       location.Manifest(),
		}
		providerManifest.ServiceVersionBindings = append(providerManifest.ServiceVersionBindings, bindingManifest)
		n.providerBindings = append(n.providerBindings, &providerBindingInfo{
			provider:         provider,
			providerManifest: providerManifest,
			binding:          binding,
			bindingManifest:  bindingManifest,
			service:          serviceInfo,
			endpoints:        endpoints,
			location:         location,
		})
	}
	sort.Slice(providerManifest.ServiceVersionBindings, func(i, j int) bool {
		left, right := providerManifest.ServiceVersionBindings[i], providerManifest.ServiceVersionBindings[j]
		if left.GetServiceLineage().GetId() != right.GetServiceLineage().GetId() {
			return left.GetServiceLineage().GetId() < right.GetServiceLineage().GetId()
		}
		if left.GetBinding().GetInterfaceMajorVersion() != right.GetBinding().GetInterfaceMajorVersion() {
			return left.GetBinding().GetInterfaceMajorVersion() < right.GetBinding().GetInterfaceMajorVersion()
		}
		return left.GetBinding().GetService().GetFqn() < right.GetBinding().GetService().GetFqn()
	})
	for _, binding := range providerManifest.ServiceVersionBindings {
		if binding.GetMaximumApplicationPayloadBytes() > providerManifest.MaximumApplicationPayloadBytes {
			providerManifest.MaximumApplicationPayloadBytes = binding.GetMaximumApplicationPayloadBytes()
			providerManifest.MaximumSomeipMessageBytes = binding.GetMaximumSomeipMessageBytes()
		}
	}
	return providerManifest
}

func (n *normalizer) resolveService(reference *optionsv1alpha1.ProtobufReference, location descriptor.Location) protoreflect.ServiceDescriptor {
	name, ok := normalizedFQN(reference)
	if !ok {
		n.diagnostics.error(ruleProfile, location, "service reference must use a leading-dot fully qualified protobuf name")
		return nil
	}
	descriptor, err := n.set.Files.FindDescriptorByName(name)
	if err != nil {
		n.diagnostics.error(ruleProfile, location, fmt.Sprintf("service reference %q cannot be resolved", reference.GetFqn()))
		return nil
	}
	service, ok := descriptor.(protoreflect.ServiceDescriptor)
	if !ok {
		n.diagnostics.error(ruleProfile, location, fmt.Sprintf("reference %q does not name a protobuf service", reference.GetFqn()))
		return nil
	}
	return service
}

func (n *normalizer) validateMemberEndpointOverrides(
	service *serviceInfo,
	binding *optionsv1alpha1.ServiceVersionBinding,
	endpoints map[string]*optionsv1alpha1.Endpoint,
	location descriptor.Location,
) {
	members := make(map[uint32]struct{}, len(service.manifest.GetMembers()))
	for _, member := range service.manifest.GetMembers() {
		members[member.GetMemberId()] = struct{}{}
	}
	seen := make(map[uint32]struct{})
	for _, override := range binding.GetMemberEndpointOverrides() {
		if _, found := seen[override.GetMemberId()]; found {
			n.diagnostics.error(ruleProfile, location, fmt.Sprintf("member endpoint override for member_id %d is duplicated", override.GetMemberId()))
		}
		seen[override.GetMemberId()] = struct{}{}
		if _, found := members[override.GetMemberId()]; !found {
			n.diagnostics.error(ruleProfile, location, fmt.Sprintf("member endpoint override references unknown member_id %d", override.GetMemberId()))
		}
		if _, found := endpoints[override.GetEndpointName()]; !found {
			n.diagnostics.error(ruleProfile, location, fmt.Sprintf("member endpoint override references undeclared endpoint %q", override.GetEndpointName()))
		}
	}
}

func (n *normalizer) sortManifestContents() {
	services := make([]*manifestv1alpha1.ServiceManifest, 0, len(n.services))
	for _, service := range n.services {
		services = append(services, service.manifest)
	}
	sort.Slice(services, func(i, j int) bool {
		return services[i].GetService().GetFqn() < services[j].GetService().GetFqn()
	})
	n.manifest.Services = services
}

func validateServiceID(diagnostics *diagnosticCollector, location descriptor.Location, identifier uint32) {
	if identifier > 0xffff || identifier == 0 || identifier == 0x0101 || identifier == 0x433f || identifier == 0xfffe || identifier == 0xffff {
		diagnostics.error(ruleID, location, fmt.Sprintf("service_id %d is not a permitted concrete SOME/IP service identifier", identifier))
	}
}

func validateInstanceID(diagnostics *diagnosticCollector, location descriptor.Location, identifier uint32) {
	if identifier > 0xffff || identifier == 0 || identifier == 0xffff {
		diagnostics.error(ruleID, location, fmt.Sprintf("instance_id %d is not a permitted concrete SOME/IP instance identifier", identifier))
	}
}

func validateMemberID(diagnostics *diagnosticCollector, location descriptor.Location, identifier uint32, isEvent bool) {
	if identifier > 0xffff || identifier == 0 || identifier == 0x7fff || identifier == 0x8000 || identifier == 0xffff {
		diagnostics.error(ruleID, location, fmt.Sprintf("member identifier %d is reserved or outside the 16-bit SOME/IP range", identifier))
		return
	}
	if isEvent && (identifier < 0x8000 || identifier > 0x8fff) {
		diagnostics.warning(ruleIDRecommendation, location, fmt.Sprintf("event or notifier identifier %d is outside the recommended 0x8000-0x8fff range", identifier))
	}
	if !isEvent && identifier > 0x7fff {
		diagnostics.warning(ruleIDRecommendation, location, fmt.Sprintf("method identifier %d is outside the recommended 0x0001-0x7ffe range", identifier))
	}
}

func validateEventGroupID(diagnostics *diagnosticCollector, location descriptor.Location, identifier uint32) {
	if identifier > 0xffff || identifier == 0 || identifier == 0xffff {
		diagnostics.error(ruleID, location, fmt.Sprintf("event group identifier %d is reserved or outside the 16-bit SOME/IP range", identifier))
	}
}

func validateEndpoint(diagnostics *diagnosticCollector, location descriptor.Location, endpoint *optionsv1alpha1.Endpoint) {
	address, err := netip.ParseAddr(endpoint.GetAddress())
	if err != nil || address.String() != endpoint.GetAddress() {
		diagnostics.error(ruleProfile, location, fmt.Sprintf("endpoint address %q is not a canonical IPv4 or IPv6 literal", endpoint.GetAddress()))
	}
	if endpoint.GetTransport() != optionsv1alpha1.TransportProtocol_TRANSPORT_PROTOCOL_UDP && endpoint.GetTransport() != optionsv1alpha1.TransportProtocol_TRANSPORT_PROTOCOL_TCP {
		diagnostics.error(ruleProfile, location, fmt.Sprintf("endpoint %q must use UDP or TCP", endpoint.GetName()))
	}
	if endpoint.GetPort() == 0 || endpoint.GetPort() > 65535 {
		diagnostics.error(ruleProfile, location, fmt.Sprintf("endpoint %q port %d is outside the valid transport-port range", endpoint.GetName(), endpoint.GetPort()))
	}
}

func validateSemanticVersion(diagnostics *diagnosticCollector, location descriptor.Location, version *optionsv1alpha1.SemanticVersion) {
	if !validSemanticVersionIdentifiers(version.GetPrerelease(), true) {
		diagnostics.error(ruleProfile, location, fmt.Sprintf("profile prerelease %q is not a valid Semantic Version pre-release identifier", version.GetPrerelease()))
	}
	if !validSemanticVersionIdentifiers(version.GetBuildMetadata(), false) {
		diagnostics.error(ruleProfile, location, fmt.Sprintf("profile build_metadata %q is not a valid Semantic Version build identifier", version.GetBuildMetadata()))
	}
}

func validSemanticVersionIdentifiers(value string, rejectLeadingZeroNumeric bool) bool {
	if value == "" {
		return true
	}
	for _, identifier := range strings.Split(value, ".") {
		if identifier == "" {
			return false
		}
		numeric := true
		for _, character := range identifier {
			if !((character >= '0' && character <= '9') || (character >= 'A' && character <= 'Z') || (character >= 'a' && character <= 'z') || character == '-') {
				return false
			}
			if character < '0' || character > '9' {
				numeric = false
			}
		}
		if rejectLeadingZeroNumeric && numeric && len(identifier) > 1 && identifier[0] == '0' {
			return false
		}
	}
	return true
}

func endpointTuple(endpoint *optionsv1alpha1.Endpoint) string {
	return strings.Join([]string{endpoint.GetTransport().String(), endpoint.GetAddress(), fmt.Sprint(endpoint.GetPort())}, "/")
}

func normalizedFQN(reference *optionsv1alpha1.ProtobufReference) (protoreflect.FullName, bool) {
	if reference == nil || !strings.HasPrefix(reference.GetFqn(), ".") || reference.GetFqn() == "." {
		return "", false
	}
	name := protoreflect.FullName(strings.TrimPrefix(reference.GetFqn(), "."))
	if !name.IsValid() {
		return "", false
	}
	return name, true
}

func (n *normalizer) someipMessageSize(payload uint64, location descriptor.Location, service string) uint64 {
	messageSize, overflow := checkedAdd(payload, 16)
	if overflow {
		n.diagnostics.error(ruleProfile, location, fmt.Sprintf("service %s maximum SOME/IP message size exceeds the supported uint64 bound", service))
	}
	return messageSize
}
