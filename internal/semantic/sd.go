package semantic

import (
	"fmt"
	"net/netip"
	"sort"
	"strings"

	manifestv1alpha1 "github.com/eshenhu/someip_proto/gen/go/io/github/someip_proto/manifest/v1alpha1"
	optionsv1alpha1 "github.com/eshenhu/someip_proto/gen/go/io/github/someip_proto/options/v1alpha1"
	"github.com/eshenhu/someip_proto/internal/descriptor"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/durationpb"
)

const maximumServiceDiscoveryTTLSeconds uint32 = 0xFFFFFF

// normalizeProviderSD validates the optional static SD offer for every
// provider binding. Provider-only profiles may omit an offer; consumers cannot
// select a binding without one.
func (n *normalizer) normalizeProviderSD() {
	for _, binding := range n.providerBindings {
		n.normalizeProviderBindingSD(binding)
	}
}

func (n *normalizer) normalizeProviderBindingSD(binding *providerBindingInfo) {
	if binding == nil || binding.binding == nil || binding.bindingManifest == nil || binding.service == nil {
		return
	}
	offer := binding.binding.GetServiceDiscoveryOffer()
	if offer == nil {
		return
	}
	offer = canonicalServiceDiscoveryOffer(offer)
	binding.bindingManifest.ServiceDiscoveryOffer = offer
	binding.bindingManifest.EventGroupDeliveryPolicies = cloneDeliveryPolicies(offer.GetEventGroupDeliveryPolicies())

	advertised := make(map[string]struct{}, len(offer.GetEndpointNames()))
	transportCounts := make(map[optionsv1alpha1.TransportProtocol]uint32, 2)
	addressFamily := 0
	for _, endpointName := range offer.GetEndpointNames() {
		if _, found := advertised[endpointName]; found {
			n.diagnostics.error(ruleSDTopology, binding.location, fmt.Sprintf("service discovery offer repeats endpoint %q", endpointName))
			continue
		}
		advertised[endpointName] = struct{}{}
		endpoint := binding.endpoints[endpointName]
		if endpoint == nil {
			n.diagnostics.error(ruleSDTopology, binding.location, fmt.Sprintf("service discovery offer references undeclared provider endpoint %q", endpointName))
			continue
		}
		transportCounts[endpoint.GetTransport()]++
		if transportCounts[endpoint.GetTransport()] > 1 {
			n.diagnostics.error(ruleSDTopology, binding.location, fmt.Sprintf("service discovery offer declares more than one %s endpoint", endpoint.GetTransport().String()))
		}
		address, valid := canonicalIPAddress(endpoint.GetAddress())
		if !valid || address.IsMulticast() || address.IsUnspecified() {
			n.diagnostics.error(ruleSDTopology, binding.location, fmt.Sprintf("service discovery offer endpoint %q must use a canonical unicast IPv4 or IPv6 address", endpointName))
			continue
		}
		family := ipAddressFamily(address)
		if addressFamily == 0 {
			addressFamily = family
		} else if addressFamily != family {
			n.diagnostics.error(ruleSDTopology, binding.location, "service discovery offer endpoints must use one common IP address family")
		}
	}
	if len(advertised) == 0 {
		n.diagnostics.error(ruleSDTopology, binding.location, "service discovery offer must declare at least one provider endpoint")
	}
	for _, member := range binding.service.manifest.GetMembers() {
		endpoint := providerEndpointForMember(binding, member.GetMemberId())
		if endpoint == nil {
			n.diagnostics.error(ruleSDTopology, binding.location, fmt.Sprintf("service discovery offer cannot resolve an effective endpoint for member_id %d", member.GetMemberId()))
			continue
		}
		if _, found := advertised[endpoint.GetName()]; !found {
			n.diagnostics.error(ruleSDTopology, binding.location, fmt.Sprintf("service discovery offer does not advertise effective endpoint %q for member_id %d", endpoint.GetName(), member.GetMemberId()))
		}
	}
	n.validateServiceDiscoveryTiming(offer.GetTiming(), true, "service discovery offer", binding.location)
	n.validateProviderDeliveryPolicies(binding, offer)
}

func (n *normalizer) validateProviderDeliveryPolicies(binding *providerBindingInfo, offer *optionsv1alpha1.ServiceDiscoveryOffer) {
	if binding == nil || binding.service == nil || offer == nil {
		return
	}
	eventGroups := eventGroupIDs(binding.service)
	seen := make(map[string]struct{}, len(offer.GetEventGroupDeliveryPolicies()))
	for _, policy := range offer.GetEventGroupDeliveryPolicies() {
		if policy == nil {
			n.diagnostics.error(ruleSDDelivery, binding.location, "service discovery offer contains an empty event-group delivery policy")
			continue
		}
		name := policy.GetEventGroupName()
		if _, found := seen[name]; found {
			n.diagnostics.error(ruleSDDelivery, binding.location, fmt.Sprintf("service discovery offer repeats delivery policy for event group %q", name))
		}
		seen[name] = struct{}{}
		if _, found := eventGroups[name]; !found {
			n.diagnostics.error(ruleSDDelivery, binding.location, fmt.Sprintf("service discovery offer declares delivery policy for unknown event group %q", name))
		}
		n.validateDeliveryPolicy(policy, binding.location)
	}
	for name := range eventGroups {
		if _, found := seen[name]; !found {
			n.diagnostics.error(ruleSDDelivery, binding.location, fmt.Sprintf("service discovery offer must declare a delivery policy for event group %q", name))
		}
	}
}

func (n *normalizer) validateDeliveryPolicy(policy *optionsv1alpha1.EventGroupDeliveryPolicy, location descriptor.Location) {
	if policy == nil {
		return
	}
	switch policy.GetMode() {
	case optionsv1alpha1.EventGroupDeliveryMode_EVENT_GROUP_DELIVERY_MODE_UNICAST:
		if policy.GetMulticastEndpoint() != nil {
			n.diagnostics.error(ruleSDDelivery, location, fmt.Sprintf("unicast event group %q must not declare a multicast endpoint", policy.GetEventGroupName()))
		}
	case optionsv1alpha1.EventGroupDeliveryMode_EVENT_GROUP_DELIVERY_MODE_MULTICAST:
		endpoint := policy.GetMulticastEndpoint()
		if endpoint == nil {
			n.diagnostics.error(ruleSDDelivery, location, fmt.Sprintf("multicast event group %q must declare a multicast endpoint", policy.GetEventGroupName()))
			return
		}
		address, valid := canonicalIPAddress(endpoint.GetAddress())
		if !valid || !address.IsMulticast() {
			n.diagnostics.error(ruleSDDelivery, location, fmt.Sprintf("multicast event group %q must use a canonical multicast IPv4 or IPv6 address", policy.GetEventGroupName()))
		}
		if !validPort(endpoint.GetPort()) {
			n.diagnostics.error(ruleSDDelivery, location, fmt.Sprintf("multicast event group %q port %d is outside the valid transport-port range", policy.GetEventGroupName(), endpoint.GetPort()))
		}
	default:
		n.diagnostics.error(ruleSDDelivery, location, fmt.Sprintf("event group %q must declare UNICAST or MULTICAST delivery", policy.GetEventGroupName()))
	}
}

// normalizeConsumerBindings resolves consumer selectors only after provider TP
// and SD facts have been normalized, so declaration order cannot affect the
// derived manifest.
func (n *normalizer) normalizeConsumerBindings(
	definition profileDefinition,
	manifest *manifestv1alpha1.ProfileManifest,
	profile *optionsv1alpha1.DeploymentProfile,
) {
	if manifest == nil || profile == nil {
		return
	}
	location := n.fileOptionsLocation(definition)
	consumerNames := make(map[string]struct{}, len(profile.GetConsumerBindings()))
	for _, authored := range profile.GetConsumerBindings() {
		binding := canonicalConsumerServiceBinding(authored)
		if binding == nil {
			n.diagnostics.error(ruleSDTopology, location, "deployment profile contains an empty consumer binding")
			continue
		}
		if !profileIdentifierPattern.MatchString(binding.GetConsumerName()) {
			n.diagnostics.error(ruleSDTopology, location, fmt.Sprintf("consumer_name %q must be a lowercase ASCII identifier of at most 128 characters", binding.GetConsumerName()))
		}
		if _, found := consumerNames[binding.GetConsumerName()]; found {
			n.diagnostics.error(ruleSDTopology, location, fmt.Sprintf("consumer_name %q is not unique within the profile", binding.GetConsumerName()))
		}
		consumerNames[binding.GetConsumerName()] = struct{}{}
		manifest.ConsumerBindings = append(manifest.ConsumerBindings, n.normalizeConsumerBinding(binding, location))
	}
	sort.Slice(manifest.ConsumerBindings, func(i, j int) bool {
		left, right := manifest.ConsumerBindings[i].GetBinding(), manifest.ConsumerBindings[j].GetBinding()
		if left.GetConsumerName() != right.GetConsumerName() {
			return left.GetConsumerName() < right.GetConsumerName()
		}
		if left.GetService().GetFqn() != right.GetService().GetFqn() {
			return left.GetService().GetFqn() < right.GetService().GetFqn()
		}
		return left.GetInterfaceMajorVersion() < right.GetInterfaceMajorVersion()
	})
}

func (n *normalizer) normalizeConsumerBinding(
	binding *optionsv1alpha1.ConsumerServiceBinding,
	location descriptor.Location,
) *manifestv1alpha1.ConsumerServiceBindingManifest {
	manifest := &manifestv1alpha1.ConsumerServiceBindingManifest{
		Binding:        binding,
		LocalEndpoints: binding.GetLocalEndpoints(),
		FindTiming:     binding.GetFindTiming(),
		Location:       location.Manifest(),
	}
	service := n.resolveConsumerService(binding.GetService(), location)
	endpoints := n.validateConsumerEndpoints(binding, location)
	n.validateConsumerDefaultEndpoint(binding, endpoints, location)
	n.validateServiceDiscoveryTiming(binding.GetFindTiming(), false, "consumer find timing", location)
	if service == nil {
		return manifest
	}
	if binding.GetServiceLineage().GetId() == "" {
		n.diagnostics.error(ruleSDTopology, location, "consumer binding must declare a nonempty service lineage")
	} else if binding.GetServiceLineage().GetId() != service.options.GetServiceLineage().GetId() {
		n.diagnostics.error(ruleSDTopology, location, fmt.Sprintf("consumer service lineage %q does not match service lineage %q", binding.GetServiceLineage().GetId(), service.options.GetServiceLineage().GetId()))
	}
	if binding.GetInterfaceMajorVersion() != service.options.GetInterfaceMajorVersion() {
		n.diagnostics.error(ruleSDTopology, location, fmt.Sprintf("consumer major version %d does not match service major version %d", binding.GetInterfaceMajorVersion(), service.options.GetInterfaceMajorVersion()))
	}
	n.validateConsumerMemberEndpointOverrides(service, binding, endpoints, location)
	n.validateConsumerRequestEndpointCoverage(service, binding, endpoints, location)

	candidates, wildcard := n.resolveConsumerProviderSelector(binding, service, location)
	manifest.ProviderSelector = &manifestv1alpha1.ResolvedProviderSelectorManifest{
		Selector:   cloneProviderSelector(binding.GetProviderSelector()),
		Candidates: providerCandidateManifests(candidates),
	}
	for _, candidate := range candidates {
		if candidate.bindingManifest.GetServiceDiscoveryOffer() == nil {
			n.diagnostics.error(ruleSDTopology, location, fmt.Sprintf("selected provider %q has no static service discovery offer for %s", candidate.provider.GetProviderName(), binding.GetService().GetFqn()))
		}
	}
	n.validateConsumerEndpointCompatibility(binding, endpoints, service, candidates, location)
	manifest.InheritedMemberTpAnalysis = n.inheritedTPAnalysis(binding, endpoints, candidates, wildcard, location)
	manifest.Subscriptions = n.normalizeConsumerSubscriptions(binding, service, endpoints, candidates, location)
	return manifest
}

func (n *normalizer) resolveConsumerService(reference *optionsv1alpha1.ProtobufReference, location descriptor.Location) *serviceInfo {
	name, valid := normalizedFQN(reference)
	if !valid {
		n.diagnostics.error(ruleSDTopology, location, "consumer service reference must use a leading-dot fully qualified protobuf name")
		return nil
	}
	resolved, err := n.set.Files.FindDescriptorByName(name)
	if err != nil {
		n.diagnostics.error(ruleSDTopology, location, fmt.Sprintf("consumer service reference %q cannot be resolved", reference.GetFqn()))
		return nil
	}
	service, valid := resolved.(protoreflect.ServiceDescriptor)
	if !valid {
		n.diagnostics.error(ruleSDTopology, location, fmt.Sprintf("consumer reference %q does not name a protobuf service", reference.GetFqn()))
		return nil
	}
	return n.normalizeService(service)
}

func (n *normalizer) validateConsumerEndpoints(
	binding *optionsv1alpha1.ConsumerServiceBinding,
	location descriptor.Location,
) map[string]*optionsv1alpha1.ConsumerEndpoint {
	endpoints := make(map[string]*optionsv1alpha1.ConsumerEndpoint, len(binding.GetLocalEndpoints()))
	tuples := make(map[string]struct{}, len(binding.GetLocalEndpoints()))
	for _, endpoint := range binding.GetLocalEndpoints() {
		if endpoint == nil {
			n.diagnostics.error(ruleSDTopology, location, "consumer binding contains an empty local endpoint")
			continue
		}
		if !profileIdentifierPattern.MatchString(endpoint.GetName()) {
			n.diagnostics.error(ruleSDTopology, location, fmt.Sprintf("consumer endpoint name %q must be a lowercase ASCII identifier of at most 128 characters", endpoint.GetName()))
		}
		if _, found := endpoints[endpoint.GetName()]; found {
			n.diagnostics.error(ruleSDTopology, location, fmt.Sprintf("consumer endpoint name %q is duplicated", endpoint.GetName()))
		}
		endpoints[endpoint.GetName()] = endpoint
		address, valid := canonicalIPAddress(endpoint.GetAddress())
		if !valid || address.IsMulticast() || address.IsUnspecified() {
			n.diagnostics.error(ruleSDTopology, location, fmt.Sprintf("consumer endpoint %q must use a canonical unicast IPv4 or IPv6 address", endpoint.GetName()))
		}
		if endpoint.GetTransport() != optionsv1alpha1.TransportProtocol_TRANSPORT_PROTOCOL_UDP && endpoint.GetTransport() != optionsv1alpha1.TransportProtocol_TRANSPORT_PROTOCOL_TCP {
			n.diagnostics.error(ruleSDTopology, location, fmt.Sprintf("consumer endpoint %q must use UDP or TCP", endpoint.GetName()))
		}
		portDescription := ""
		switch port := endpoint.GetPort().(type) {
		case *optionsv1alpha1.ConsumerEndpoint_FixedPort:
			if !validPort(port.FixedPort) {
				n.diagnostics.error(ruleSDTopology, location, fmt.Sprintf("consumer endpoint %q fixed port %d is outside the valid transport-port range", endpoint.GetName(), port.FixedPort))
			}
			portDescription = fmt.Sprintf("fixed:%d", port.FixedPort)
		case *optionsv1alpha1.ConsumerEndpoint_DynamicPortRange:
			rangeValue := port.DynamicPortRange
			if rangeValue == nil || !validPort(rangeValue.GetMinimumPort()) || !validPort(rangeValue.GetMaximumPort()) || rangeValue.GetMinimumPort() > rangeValue.GetMaximumPort() {
				n.diagnostics.error(ruleSDTopology, location, fmt.Sprintf("consumer endpoint %q dynamic port range must be an increasing inclusive range within 1..65535", endpoint.GetName()))
			}
			portDescription = fmt.Sprintf("dynamic:%d-%d", rangeValue.GetMinimumPort(), rangeValue.GetMaximumPort())
		default:
			n.diagnostics.error(ruleSDTopology, location, fmt.Sprintf("consumer endpoint %q must declare a fixed port or dynamic port range", endpoint.GetName()))
		}
		tuple := strings.Join([]string{endpoint.GetTransport().String(), endpoint.GetAddress(), portDescription}, "/")
		if _, found := tuples[tuple]; found {
			n.diagnostics.error(ruleSDTopology, location, fmt.Sprintf("consumer endpoint %q repeats a local endpoint tuple", endpoint.GetName()))
		}
		tuples[tuple] = struct{}{}
	}
	return endpoints
}

func (n *normalizer) validateConsumerDefaultEndpoint(
	binding *optionsv1alpha1.ConsumerServiceBinding,
	endpoints map[string]*optionsv1alpha1.ConsumerEndpoint,
	location descriptor.Location,
) {
	defaultName := binding.GetDefaultEndpointName()
	if len(endpoints) == 0 {
		if defaultName != "" {
			n.diagnostics.error(ruleSDTopology, location, fmt.Sprintf("consumer default endpoint %q is not declared", defaultName))
		}
		return
	}
	if defaultName == "" {
		n.diagnostics.error(ruleSDTopology, location, "consumer binding with local endpoints must declare default_endpoint_name")
		return
	}
	if _, found := endpoints[defaultName]; !found {
		n.diagnostics.error(ruleSDTopology, location, fmt.Sprintf("consumer default endpoint %q is not declared", defaultName))
	}
}

func (n *normalizer) validateConsumerMemberEndpointOverrides(
	service *serviceInfo,
	binding *optionsv1alpha1.ConsumerServiceBinding,
	endpoints map[string]*optionsv1alpha1.ConsumerEndpoint,
	location descriptor.Location,
) {
	if service == nil || binding == nil {
		return
	}
	members := make(map[uint32]struct{}, len(service.manifest.GetMembers()))
	for _, member := range service.manifest.GetMembers() {
		members[member.GetMemberId()] = struct{}{}
	}
	seen := make(map[uint32]struct{}, len(binding.GetMemberEndpointOverrides()))
	for _, override := range binding.GetMemberEndpointOverrides() {
		if override == nil {
			n.diagnostics.error(ruleSDTopology, location, "consumer binding contains an empty member endpoint override")
			continue
		}
		if _, found := seen[override.GetMemberId()]; found {
			n.diagnostics.error(ruleSDTopology, location, fmt.Sprintf("consumer member endpoint override for member_id %d is duplicated", override.GetMemberId()))
		}
		seen[override.GetMemberId()] = struct{}{}
		if _, found := members[override.GetMemberId()]; !found {
			n.diagnostics.error(ruleSDTopology, location, fmt.Sprintf("consumer member endpoint override references unknown member_id %d", override.GetMemberId()))
		}
		if _, found := endpoints[override.GetEndpointName()]; !found {
			n.diagnostics.error(ruleSDTopology, location, fmt.Sprintf("consumer member endpoint override references undeclared local endpoint %q", override.GetEndpointName()))
		}
	}
}

func (n *normalizer) validateConsumerRequestEndpointCoverage(
	service *serviceInfo,
	binding *optionsv1alpha1.ConsumerServiceBinding,
	endpoints map[string]*optionsv1alpha1.ConsumerEndpoint,
	location descriptor.Location,
) {
	if service == nil || binding == nil {
		return
	}
	for _, member := range service.manifest.GetMembers() {
		if !hasMemberDirection(service.memberPayloadBounds[member.GetMemberId()], manifestv1alpha1.MemberDirection_MEMBER_DIRECTION_REQUEST) {
			continue
		}
		if consumerEndpointForMember(binding, endpoints, member.GetMemberId()) == nil {
			n.diagnostics.error(ruleSDTopology, location, fmt.Sprintf("consumer binding must resolve a local endpoint for request member_id %d", member.GetMemberId()))
		}
	}
}

func hasMemberDirection(bounds []memberDirectionalBound, direction manifestv1alpha1.MemberDirection) bool {
	for _, bound := range bounds {
		if bound.direction == direction {
			return true
		}
	}
	return false
}

func (n *normalizer) resolveConsumerProviderSelector(
	binding *optionsv1alpha1.ConsumerServiceBinding,
	service *serviceInfo,
	location descriptor.Location,
) ([]*providerBindingInfo, bool) {
	if binding == nil || service == nil {
		return nil, false
	}
	compatible := n.compatibleProviderBindings(binding, service)
	selector := binding.GetProviderSelector()
	if selector == nil {
		n.diagnostics.error(ruleSDTopology, location, "consumer binding must declare a provider selector")
		return nil, false
	}
	switch selected := selector.GetSelector().(type) {
	case *optionsv1alpha1.ProviderSelector_NamedProviderName:
		if selected.NamedProviderName == "" {
			n.diagnostics.error(ruleSDTopology, location, "named provider selector must declare a nonempty provider name")
			return nil, false
		}
		for _, candidate := range compatible {
			if candidate.provider.GetProviderName() == selected.NamedProviderName {
				return []*providerBindingInfo{candidate}, false
			}
		}
		n.diagnostics.error(ruleSDTopology, location, fmt.Sprintf("named provider selector %q does not resolve to a compatible provider binding", selected.NamedProviderName))
		return nil, false
	case *optionsv1alpha1.ProviderSelector_AnyMatchingInstance:
		if !selected.AnyMatchingInstance {
			n.diagnostics.error(ruleSDTopology, location, "wildcard provider selector must set any_matching_instance to true")
			return nil, true
		}
		switch selector.GetSelectionMode() {
		case optionsv1alpha1.ProviderSelectionMode_PROVIDER_SELECTION_MODE_ONE_MATCH:
			if len(compatible) != 1 {
				n.diagnostics.error(ruleSDTopology, location, fmt.Sprintf("ONE_MATCH provider selector resolved %d compatible provider bindings", len(compatible)))
			}
		case optionsv1alpha1.ProviderSelectionMode_PROVIDER_SELECTION_MODE_ALL_MATCHES:
			if len(compatible) == 0 {
				n.diagnostics.error(ruleSDTopology, location, "ALL_MATCHES provider selector resolved no compatible provider bindings")
			}
		default:
			n.diagnostics.error(ruleSDTopology, location, "wildcard provider selector must declare ONE_MATCH or ALL_MATCHES selection_mode")
		}
		return compatible, true
	default:
		n.diagnostics.error(ruleSDTopology, location, "consumer binding must select a named provider or any_matching_instance")
		return nil, false
	}
}

func (n *normalizer) compatibleProviderBindings(
	consumer *optionsv1alpha1.ConsumerServiceBinding,
	service *serviceInfo,
) []*providerBindingInfo {
	if consumer == nil || service == nil {
		return nil
	}
	compatible := make([]*providerBindingInfo, 0)
	for _, candidate := range n.providerBindings {
		if candidate == nil || candidate.provider == nil || candidate.binding == nil || candidate.service == nil {
			continue
		}
		if candidate.service.descriptor.FullName() != service.descriptor.FullName() {
			continue
		}
		if candidate.binding.GetInterfaceMajorVersion() != consumer.GetInterfaceMajorVersion() {
			continue
		}
		if candidate.service.options.GetServiceLineage().GetId() != consumer.GetServiceLineage().GetId() {
			continue
		}
		compatible = append(compatible, candidate)
	}
	sort.Slice(compatible, func(i, j int) bool {
		left, right := compatible[i], compatible[j]
		if left.provider.GetProviderName() != right.provider.GetProviderName() {
			return left.provider.GetProviderName() < right.provider.GetProviderName()
		}
		if left.provider.GetServiceId() != right.provider.GetServiceId() {
			return left.provider.GetServiceId() < right.provider.GetServiceId()
		}
		if left.provider.GetInstanceId() != right.provider.GetInstanceId() {
			return left.provider.GetInstanceId() < right.provider.GetInstanceId()
		}
		return left.binding.GetService().GetFqn() < right.binding.GetService().GetFqn()
	})
	return compatible
}

func providerCandidateManifests(candidates []*providerBindingInfo) []*manifestv1alpha1.ProviderCandidateManifest {
	manifests := make([]*manifestv1alpha1.ProviderCandidateManifest, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate == nil || candidate.provider == nil || candidate.binding == nil || candidate.service == nil {
			continue
		}
		manifests = append(manifests, &manifestv1alpha1.ProviderCandidateManifest{
			ProviderName:          candidate.provider.GetProviderName(),
			ServiceId:             candidate.provider.GetServiceId(),
			InstanceId:            candidate.provider.GetInstanceId(),
			Service:               cloneReference(candidate.binding.GetService().GetFqn()),
			InterfaceMajorVersion: candidate.binding.GetInterfaceMajorVersion(),
			ServiceLineage:        cloneServiceLineage(candidate.service.options.GetServiceLineage()),
		})
	}
	return manifests
}

func (n *normalizer) validateConsumerEndpointCompatibility(
	consumer *optionsv1alpha1.ConsumerServiceBinding,
	endpoints map[string]*optionsv1alpha1.ConsumerEndpoint,
	service *serviceInfo,
	candidates []*providerBindingInfo,
	location descriptor.Location,
) {
	if consumer == nil || service == nil {
		return
	}
	for _, candidate := range candidates {
		for _, member := range service.manifest.GetMembers() {
			local := consumerEndpointForMember(consumer, endpoints, member.GetMemberId())
			if local == nil {
				continue
			}
			remote := providerEndpointForMember(candidate, member.GetMemberId())
			if remote == nil {
				continue
			}
			if local.GetTransport() != remote.GetTransport() {
				n.diagnostics.error(ruleSDTopology, location, fmt.Sprintf("consumer endpoint %q transport does not match provider endpoint %q for member_id %d", local.GetName(), remote.GetName(), member.GetMemberId()))
			}
			localAddress, localValid := canonicalIPAddress(local.GetAddress())
			remoteAddress, remoteValid := canonicalIPAddress(remote.GetAddress())
			if localValid && remoteValid && ipAddressFamily(localAddress) != ipAddressFamily(remoteAddress) {
				n.diagnostics.error(ruleSDTopology, location, fmt.Sprintf("consumer endpoint %q address family does not match provider endpoint %q for member_id %d", local.GetName(), remote.GetName(), member.GetMemberId()))
			}
		}
	}
}

func (n *normalizer) inheritedTPAnalysis(
	consumer *optionsv1alpha1.ConsumerServiceBinding,
	consumerEndpoints map[string]*optionsv1alpha1.ConsumerEndpoint,
	candidates []*providerBindingInfo,
	wildcard bool,
	location descriptor.Location,
) []*manifestv1alpha1.ProviderMemberTpAnalysisManifest {
	if len(candidates) == 0 || candidates[0] == nil || candidates[0].bindingManifest == nil {
		return nil
	}
	base := candidates[0].bindingManifest.GetMemberTpAnalysis()
	if wildcard {
		for _, candidate := range candidates[1:] {
			if candidate == nil || candidate.bindingManifest == nil || !equivalentTPContracts(base, candidate.bindingManifest.GetMemberTpAnalysis()) {
				n.diagnostics.error(ruleTPWildcard, location, "wildcard provider selector candidates do not share one provider TP contract")
				break
			}
		}
	}
	inherited := cloneTPAnalyses(base)
	for _, member := range inherited {
		localEndpoint := consumerEndpointForMember(consumer, consumerEndpoints, member.GetMemberId())
		for _, direction := range member.GetDirections() {
			if direction.GetDirection() == manifestv1alpha1.MemberDirection_MEMBER_DIRECTION_REQUEST {
				direction.Endpoint = consumerEndpointManifest(localEndpoint)
			}
			if wildcard && len(candidates) > 1 && direction.GetDirection() != manifestv1alpha1.MemberDirection_MEMBER_DIRECTION_REQUEST {
				direction.Endpoint = nil
			}
		}
	}
	return inherited
}

func equivalentTPContracts(
	left, right []*manifestv1alpha1.ProviderMemberTpAnalysisManifest,
) bool {
	if len(left) != len(right) {
		return false
	}
	leftByMember := make(map[uint32]*manifestv1alpha1.ProviderMemberTpAnalysisManifest, len(left))
	rightByMember := make(map[uint32]*manifestv1alpha1.ProviderMemberTpAnalysisManifest, len(right))
	for _, member := range left {
		if member == nil {
			return false
		}
		leftByMember[member.GetMemberId()] = member
	}
	for _, member := range right {
		if member == nil {
			return false
		}
		rightByMember[member.GetMemberId()] = member
	}
	if len(leftByMember) != len(left) || len(rightByMember) != len(right) {
		return false
	}
	for memberID, leftMember := range leftByMember {
		rightMember, found := rightByMember[memberID]
		if !found || leftMember.GetTpWhenNeeded() != rightMember.GetTpWhenNeeded() || !equivalentTPDirections(leftMember.GetDirections(), rightMember.GetDirections()) {
			return false
		}
	}
	return true
}

func equivalentTPDirections(
	left, right []*manifestv1alpha1.MemberDirectionTpAnalysisManifest,
) bool {
	if len(left) != len(right) {
		return false
	}
	leftByDirection := make(map[manifestv1alpha1.MemberDirection]*manifestv1alpha1.MemberDirectionTpAnalysisManifest, len(left))
	rightByDirection := make(map[manifestv1alpha1.MemberDirection]*manifestv1alpha1.MemberDirectionTpAnalysisManifest, len(right))
	for _, direction := range left {
		if direction == nil {
			return false
		}
		leftByDirection[direction.GetDirection()] = direction
	}
	for _, direction := range right {
		if direction == nil {
			return false
		}
		rightByDirection[direction.GetDirection()] = direction
	}
	if len(leftByDirection) != len(left) || len(rightByDirection) != len(right) {
		return false
	}
	for direction, leftAnalysis := range leftByDirection {
		rightAnalysis, found := rightByDirection[direction]
		if !found || leftAnalysis.GetEffectiveTransport() != rightAnalysis.GetEffectiveTransport() {
			return false
		}
	}
	return true
}

func (n *normalizer) normalizeConsumerSubscriptions(
	binding *optionsv1alpha1.ConsumerServiceBinding,
	service *serviceInfo,
	endpoints map[string]*optionsv1alpha1.ConsumerEndpoint,
	candidates []*providerBindingInfo,
	location descriptor.Location,
) []*manifestv1alpha1.ConsumerSubscriptionManifest {
	if binding == nil || service == nil {
		return nil
	}
	eventGroups := eventGroupIDs(service)
	fieldNotifierGroups := fieldNotifierEventGroups(service)
	seen := make(map[string]struct{}, len(binding.GetSubscriptions()))
	normalized := make([]*manifestv1alpha1.ConsumerSubscriptionManifest, 0, len(binding.GetSubscriptions()))
	for _, subscription := range binding.GetSubscriptions() {
		if subscription == nil {
			n.diagnostics.error(ruleSDSubscription, location, "consumer binding contains an empty subscription")
			continue
		}
		name := subscription.GetEventGroupName()
		if _, found := seen[name]; found {
			n.diagnostics.error(ruleSDSubscription, location, fmt.Sprintf("consumer binding repeats subscription for event group %q", name))
		}
		seen[name] = struct{}{}
		groupID, found := eventGroups[name]
		if !found {
			n.diagnostics.error(ruleSDSubscription, location, fmt.Sprintf("subscription references unknown event group %q", name))
		}
		if !validServiceDiscoveryTTL(subscription.GetTtlSeconds()) {
			n.diagnostics.error(ruleSDSubscription, location, fmt.Sprintf("subscription TTL %d must be within 1..0xFFFFFF", subscription.GetTtlSeconds()))
		}
		policies := deliveryPoliciesForEventGroup(candidates, name)
		pureMulticast := len(policies) == len(candidates) && len(policies) > 0
		for _, policy := range policies {
			pureMulticast = pureMulticast && policy.GetMode() == optionsv1alpha1.EventGroupDeliveryMode_EVENT_GROUP_DELIVERY_MODE_MULTICAST
		}
		n.validateSubscriptionEndpoints(subscription, endpoints, pureMulticast, location)
		if subscription.GetRequestInitialData() && !fieldNotifierGroups[name] {
			n.diagnostics.error(ruleSDSubscription, location, fmt.Sprintf("subscription for event group %q requests initial data but the group contains no field notifier", name))
		}
		normalized = append(normalized, &manifestv1alpha1.ConsumerSubscriptionManifest{
			Subscription:         cloneSubscription(subscription),
			EventGroupId:         groupID,
			DeliveryPolicy:       uniformDeliveryPolicy(policies, len(candidates)),
			InitialDataRequested: subscription.GetRequestInitialData(),
		})
	}
	sort.Slice(normalized, func(i, j int) bool {
		left, right := normalized[i].GetSubscription(), normalized[j].GetSubscription()
		if left.GetEventGroupName() != right.GetEventGroupName() {
			return left.GetEventGroupName() < right.GetEventGroupName()
		}
		if left.GetTtlSeconds() != right.GetTtlSeconds() {
			return left.GetTtlSeconds() < right.GetTtlSeconds()
		}
		return strings.Join(left.GetLocalEndpointNames(), "\x00") < strings.Join(right.GetLocalEndpointNames(), "\x00")
	})
	return normalized
}

func (n *normalizer) validateSubscriptionEndpoints(
	subscription *optionsv1alpha1.Subscription,
	endpoints map[string]*optionsv1alpha1.ConsumerEndpoint,
	pureMulticast bool,
	location descriptor.Location,
) {
	if subscription == nil {
		return
	}
	if len(subscription.GetLocalEndpointNames()) == 0 && (!pureMulticast || subscription.GetRequestInitialData()) {
		n.diagnostics.error(ruleSDSubscription, location, fmt.Sprintf("subscription for event group %q must declare a local endpoint unless every selected provider delivers multicast and no initial data is requested", subscription.GetEventGroupName()))
	}
	seen := make(map[string]struct{}, len(subscription.GetLocalEndpointNames()))
	for _, endpointName := range subscription.GetLocalEndpointNames() {
		if _, found := seen[endpointName]; found {
			n.diagnostics.error(ruleSDSubscription, location, fmt.Sprintf("subscription for event group %q repeats local endpoint %q", subscription.GetEventGroupName(), endpointName))
		}
		seen[endpointName] = struct{}{}
		if _, found := endpoints[endpointName]; !found {
			n.diagnostics.error(ruleSDSubscription, location, fmt.Sprintf("subscription for event group %q references undeclared local endpoint %q", subscription.GetEventGroupName(), endpointName))
		}
	}
}

func (n *normalizer) validateServiceDiscoveryTiming(
	timing *optionsv1alpha1.ServiceDiscoveryTiming,
	requireComplete bool,
	subject string,
	location descriptor.Location,
) {
	if timing == nil {
		n.diagnostics.error(ruleSDTiming, location, fmt.Sprintf("%s is required", subject))
		return
	}
	if !validServiceDiscoveryTTL(timing.GetLiveTtlSeconds()) {
		n.diagnostics.error(ruleSDTiming, location, fmt.Sprintf("%s live TTL %d must be within 1..0xFFFFFF", subject, timing.GetLiveTtlSeconds()))
	}
	hasDetails := timing.GetInitialDelay() != nil || timing.GetRepetitionsBaseDelay() != nil || timing.GetRepetitionsMax() != 0 || timing.GetRequestResponseDelay() != nil || timing.GetCyclicOfferDelay() != nil
	if !requireComplete && !hasDetails {
		return
	}
	if timing.GetInitialDelay() == nil {
		n.diagnostics.error(ruleSDTiming, location, fmt.Sprintf("%s must declare initial_delay when timing details are present", subject))
	} else {
		n.validateDurationRange(timing.GetInitialDelay(), "initial_delay", subject, location)
	}
	if !validSDDuration(timing.GetRepetitionsBaseDelay()) {
		n.diagnostics.error(ruleSDTiming, location, fmt.Sprintf("%s repetitions_base_delay must be a nonnegative integral-millisecond duration", subject))
	}
	if timing.GetRepetitionsMax() == 0 {
		n.diagnostics.error(ruleSDTiming, location, fmt.Sprintf("%s repetitions_max must be nonzero when timing details are present", subject))
	}
	if timing.GetRequestResponseDelay() == nil {
		n.diagnostics.error(ruleSDTiming, location, fmt.Sprintf("%s must declare request_response_delay when timing details are present", subject))
	} else {
		n.validateDurationRange(timing.GetRequestResponseDelay(), "request_response_delay", subject, location)
	}
	if !validSDDuration(timing.GetCyclicOfferDelay()) {
		n.diagnostics.error(ruleSDTiming, location, fmt.Sprintf("%s cyclic_offer_delay must be a nonnegative integral-millisecond duration", subject))
	}
}

func (n *normalizer) validateDurationRange(
	rangeValue *optionsv1alpha1.DurationRange,
	name, subject string,
	location descriptor.Location,
) {
	minimum, minimumValid := milliseconds(rangeValue.GetMinimum())
	maximum, maximumValid := milliseconds(rangeValue.GetMaximum())
	if !minimumValid || !maximumValid {
		n.diagnostics.error(ruleSDTiming, location, fmt.Sprintf("%s %s must declare nonnegative integral-millisecond minimum and maximum durations", subject, name))
		return
	}
	if minimum > maximum {
		n.diagnostics.error(ruleSDTiming, location, fmt.Sprintf("%s %s minimum must not exceed maximum", subject, name))
	}
}

func validServiceDiscoveryTTL(ttl uint32) bool {
	return ttl >= 1 && ttl <= maximumServiceDiscoveryTTLSeconds
}

func validSDDuration(duration *durationpb.Duration) bool {
	_, valid := milliseconds(duration)
	return valid
}

func milliseconds(duration *durationpb.Duration) (int64, bool) {
	if duration == nil {
		return 0, false
	}
	seconds, nanos := duration.GetSeconds(), duration.GetNanos()
	if seconds < 0 || seconds > 315576000000 || nanos < 0 || nanos >= 1000000000 || nanos%1000000 != 0 {
		return 0, false
	}
	return seconds*1000 + int64(nanos)/1000000, true
}

func eventGroupIDs(service *serviceInfo) map[string]uint32 {
	groups := make(map[string]uint32)
	if service == nil || service.manifest == nil {
		return groups
	}
	for _, group := range service.manifest.GetEventGroups() {
		groups[group.GetName()] = group.GetId()
	}
	return groups
}

func fieldNotifierEventGroups(service *serviceInfo) map[string]bool {
	groups := make(map[string]bool)
	if service == nil || service.manifest == nil {
		return groups
	}
	for _, member := range service.manifest.GetMembers() {
		if member.GetKind() != "field_notifier" {
			continue
		}
		for _, group := range member.GetEventGroupNames() {
			groups[group] = true
		}
	}
	return groups
}

func consumerEndpointForMember(
	binding *optionsv1alpha1.ConsumerServiceBinding,
	endpoints map[string]*optionsv1alpha1.ConsumerEndpoint,
	memberID uint32,
) *optionsv1alpha1.ConsumerEndpoint {
	if binding == nil {
		return nil
	}
	endpoint := endpoints[binding.GetDefaultEndpointName()]
	for _, override := range binding.GetMemberEndpointOverrides() {
		if override.GetMemberId() != memberID {
			continue
		}
		if resolved := endpoints[override.GetEndpointName()]; resolved != nil {
			endpoint = resolved
		}
		break
	}
	return endpoint
}

func deliveryPoliciesForEventGroup(candidates []*providerBindingInfo, eventGroup string) []*optionsv1alpha1.EventGroupDeliveryPolicy {
	policies := make([]*optionsv1alpha1.EventGroupDeliveryPolicy, 0, len(candidates))
	for _, candidate := range candidates {
		policy := providerDeliveryPolicy(candidate, eventGroup)
		if policy == nil {
			continue
		}
		policies = append(policies, policy)
	}
	return policies
}

func providerDeliveryPolicy(binding *providerBindingInfo, eventGroup string) *optionsv1alpha1.EventGroupDeliveryPolicy {
	if binding == nil || binding.bindingManifest == nil || binding.bindingManifest.GetServiceDiscoveryOffer() == nil {
		return nil
	}
	for _, policy := range binding.bindingManifest.GetServiceDiscoveryOffer().GetEventGroupDeliveryPolicies() {
		if policy.GetEventGroupName() == eventGroup {
			return policy
		}
	}
	return nil
}

func uniformDeliveryPolicy(
	policies []*optionsv1alpha1.EventGroupDeliveryPolicy,
	candidateCount int,
) *optionsv1alpha1.EventGroupDeliveryPolicy {
	if len(policies) == 0 || len(policies) != candidateCount {
		return nil
	}
	first := policies[0]
	for _, policy := range policies[1:] {
		if !proto.Equal(first, policy) {
			return nil
		}
	}
	return canonicalEventGroupDeliveryPolicy(first)
}

func cloneDeliveryPolicies(values []*optionsv1alpha1.EventGroupDeliveryPolicy) []*optionsv1alpha1.EventGroupDeliveryPolicy {
	clones := make([]*optionsv1alpha1.EventGroupDeliveryPolicy, 0, len(values))
	for _, value := range values {
		if value == nil {
			clones = append(clones, nil)
			continue
		}
		clones = append(clones, canonicalEventGroupDeliveryPolicy(value))
	}
	return clones
}

func cloneProviderSelector(value *optionsv1alpha1.ProviderSelector) *optionsv1alpha1.ProviderSelector {
	if value == nil {
		return nil
	}
	return proto.Clone(value).(*optionsv1alpha1.ProviderSelector)
}

func cloneSubscription(value *optionsv1alpha1.Subscription) *optionsv1alpha1.Subscription {
	if value == nil {
		return nil
	}
	cloned := proto.Clone(value).(*optionsv1alpha1.Subscription)
	cloned.LocalEndpointNames = canonicalStrings(cloned.GetLocalEndpointNames())
	return cloned
}

func cloneServiceLineage(value *optionsv1alpha1.ServiceLineage) *optionsv1alpha1.ServiceLineage {
	if value == nil {
		return nil
	}
	return proto.Clone(value).(*optionsv1alpha1.ServiceLineage)
}

func cloneTPAnalyses(values []*manifestv1alpha1.ProviderMemberTpAnalysisManifest) []*manifestv1alpha1.ProviderMemberTpAnalysisManifest {
	clones := make([]*manifestv1alpha1.ProviderMemberTpAnalysisManifest, 0, len(values))
	for _, value := range values {
		if value == nil {
			clones = append(clones, nil)
			continue
		}
		clones = append(clones, proto.Clone(value).(*manifestv1alpha1.ProviderMemberTpAnalysisManifest))
	}
	return clones
}

func consumerEndpointManifest(endpoint *optionsv1alpha1.ConsumerEndpoint) *manifestv1alpha1.EffectiveEndpointManifest {
	if endpoint == nil {
		return nil
	}
	manifest := &manifestv1alpha1.EffectiveEndpointManifest{
		EndpointName: endpoint.GetName(),
		Address:      endpoint.GetAddress(),
		Transport:    endpoint.GetTransport(),
	}
	switch port := endpoint.GetPort().(type) {
	case *optionsv1alpha1.ConsumerEndpoint_FixedPort:
		manifest.Port = &manifestv1alpha1.EffectiveEndpointManifest_FixedPort{FixedPort: port.FixedPort}
	case *optionsv1alpha1.ConsumerEndpoint_DynamicPortRange:
		if port.DynamicPortRange != nil {
			manifest.Port = &manifestv1alpha1.EffectiveEndpointManifest_DynamicPortRange{
				DynamicPortRange: proto.Clone(port.DynamicPortRange).(*optionsv1alpha1.DynamicPortRange),
			}
		}
	}
	return manifest
}

func canonicalIPAddress(value string) (netip.Addr, bool) {
	address, err := netip.ParseAddr(value)
	if err != nil || address.String() != value {
		return netip.Addr{}, false
	}
	return address, true
}

func ipAddressFamily(address netip.Addr) int {
	if address.Is4() {
		return 4
	}
	return 6
}

func validPort(port uint32) bool {
	return port >= 1 && port <= 65535
}
