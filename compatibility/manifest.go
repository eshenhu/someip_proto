package compatibility

import (
	"fmt"
	"sort"
	"strings"

	manifestv1alpha1 "github.com/eshenhu/someip_proto/gen/go/io/github/someip_proto/manifest/v1alpha1"
	optionsv1alpha1 "github.com/eshenhu/someip_proto/gen/go/io/github/someip_proto/options/v1alpha1"
	"google.golang.org/protobuf/proto"
)

// CompareManifests compares two already-normalized manifests. It does not
// mutate either argument and returns deterministically ordered diagnostics.
func CompareManifests(baseline, current *manifestv1alpha1.Manifest) []*manifestv1alpha1.Diagnostic {
	if baseline == nil || current == nil {
		return []*manifestv1alpha1.Diagnostic{compatibilityDiagnostic(nil, "both baseline and current manifests are required for comparison")}
	}
	if baseline.GetProfile() == nil || current.GetProfile() == nil {
		location := sourceLocation(current.GetProfile())
		return []*manifestv1alpha1.Diagnostic{compatibilityDiagnostic(location, "both baseline and current manifests must contain the selected deployment profile")}
	}

	comparison := manifestComparison{
		baseline: baseline,
		current:  current,
	}
	comparison.compare()
	return sortDiagnostics(comparison.diagnostics)
}

type manifestComparison struct {
	baseline               *manifestv1alpha1.Manifest
	current                *manifestv1alpha1.Manifest
	diagnostics            []*manifestv1alpha1.Diagnostic
	profileNeed            changeLevel
	reportedPayloadChanges map[string]struct{}
	reportedEnumChanges    map[string]struct{}
}

type contractChange struct {
	level    changeLevel
	location *manifestv1alpha1.SourceLocation
	message  string
}

func (c *manifestComparison) compare() {
	c.reportedPayloadChanges = make(map[string]struct{})
	c.reportedEnumChanges = make(map[string]struct{})
	c.profileNeed = maximumChange(c.profileNeed, c.classifyProfileChange())
	payloadChanges := c.classifyPayloadChanges()
	enumChanges := c.classifyEnumChanges()
	c.compareServices(payloadChanges, enumChanges)
	c.requireProfileVersion(c.profileNeed)
}

func (c *manifestComparison) classifyProfileChange() changeLevel {
	baseline := c.baseline.GetProfile()
	current := c.current.GetProfile()
	if baseline.GetProfileName() != current.GetProfileName() {
		return changeMajor
	}
	if baseline.GetUdpApplicationPayloadBudgetBytes() != current.GetUdpApplicationPayloadBudgetBytes() {
		return changeMajor
	}
	baselineProviders := providersByKey(baseline.GetProviderInstances())
	currentProviders := providersByKey(current.GetProviderInstances())
	level := changeNone
	for key, baselineProvider := range baselineProviders {
		currentProvider, found := currentProviders[key]
		if !found {
			return changeMajor
		}
		level = maximumChange(level, classifyProviderChange(baselineProvider, currentProvider))
	}
	for key := range currentProviders {
		if _, found := baselineProviders[key]; !found {
			return changeMajor
		}
	}
	level = maximumChange(level, classifyConsumerBindings(baseline.GetConsumerBindings(), current.GetConsumerBindings()))
	return level
}

func providersByKey(providers []*manifestv1alpha1.ProviderInstanceManifest) map[string]*manifestv1alpha1.ProviderInstanceManifest {
	byKey := make(map[string]*manifestv1alpha1.ProviderInstanceManifest, len(providers))
	for _, provider := range providers {
		instance := provider.GetInstance()
		key := fmt.Sprintf("%05d/%05d/%s", instance.GetServiceId(), instance.GetInstanceId(), instance.GetProviderName())
		byKey[key] = provider
	}
	return byKey
}

func classifyProviderChange(baseline, current *manifestv1alpha1.ProviderInstanceManifest) changeLevel {
	if !proto.Equal(providerWithoutBindings(baseline.GetInstance()), providerWithoutBindings(current.GetInstance())) {
		return changeMajor
	}
	baselineBindings := bindingsByKey(baseline.GetServiceVersionBindings())
	currentBindings := bindingsByKey(current.GetServiceVersionBindings())
	level := changeNone
	for key, baselineBinding := range baselineBindings {
		currentBinding, found := currentBindings[key]
		if !found || !sameProviderBindingContract(baselineBinding, currentBinding) {
			return changeMajor
		}
	}
	for key := range currentBindings {
		if _, found := baselineBindings[key]; !found {
			level = maximumChange(level, changeMinor)
		}
	}
	return level
}

func sameProviderBindingContract(
	baseline, current *manifestv1alpha1.ProviderServiceVersionBindingManifest,
) bool {
	return proto.Equal(bindingContract(baseline.GetBinding()), bindingContract(current.GetBinding())) &&
		proto.Equal(providerBindingStaticContract(baseline), providerBindingStaticContract(current))
}

func providerBindingStaticContract(
	binding *manifestv1alpha1.ProviderServiceVersionBindingManifest,
) *manifestv1alpha1.ProviderServiceVersionBindingManifest {
	if binding == nil {
		return nil
	}
	clone := proto.Clone(binding).(*manifestv1alpha1.ProviderServiceVersionBindingManifest)
	clone.Binding = nil
	clone.ServiceLineage = nil
	clone.MaximumApplicationPayloadBytes = 0
	clone.MaximumSomeipMessageBytes = 0
	clone.Location = nil
	sortServiceDiscoveryOffer(clone.ServiceDiscoveryOffer)
	sortDeliveryPolicies(clone.EventGroupDeliveryPolicies)
	sortMemberTPConfigurations(clone.MemberTpConfigurations)
	sortProviderTPAnalyses(clone.MemberTpAnalysis)
	return clone
}

func providerWithoutBindings(provider *optionsv1alpha1.ProviderInstance) *optionsv1alpha1.ProviderInstance {
	if provider == nil {
		return nil
	}
	clone := proto.Clone(provider).(*optionsv1alpha1.ProviderInstance)
	clone.ServiceVersionBindings = nil
	sort.Slice(clone.Endpoints, func(i, j int) bool {
		left, right := clone.Endpoints[i], clone.Endpoints[j]
		if left.GetName() != right.GetName() {
			return left.GetName() < right.GetName()
		}
		if left.GetTransport() != right.GetTransport() {
			return left.GetTransport() < right.GetTransport()
		}
		if left.GetAddress() != right.GetAddress() {
			return left.GetAddress() < right.GetAddress()
		}
		return left.GetPort() < right.GetPort()
	})
	sortMemberEndpointOverrides(clone.MemberEndpointOverrides)
	return clone
}

func bindingContract(binding *optionsv1alpha1.ServiceVersionBinding) *optionsv1alpha1.ServiceVersionBinding {
	if binding == nil {
		return nil
	}
	clone := proto.Clone(binding).(*optionsv1alpha1.ServiceVersionBinding)
	sortMemberEndpointOverrides(clone.MemberEndpointOverrides)
	sortServiceDiscoveryOffer(clone.ServiceDiscoveryOffer)
	sortMemberTPConfigurations(clone.MemberTpConfigurations)
	return clone
}

func classifyConsumerBindings(
	baseline, current []*manifestv1alpha1.ConsumerServiceBindingManifest,
) changeLevel {
	baselineByName := consumersByName(baseline)
	currentByName := consumersByName(current)
	for name, baselineBinding := range baselineByName {
		currentBinding, found := currentByName[name]
		if !found || !proto.Equal(consumerBindingContract(baselineBinding), consumerBindingContract(currentBinding)) {
			return changeMajor
		}
	}
	for name := range currentByName {
		if _, found := baselineByName[name]; !found {
			return changeMajor
		}
	}
	return changeNone
}

func consumersByName(bindings []*manifestv1alpha1.ConsumerServiceBindingManifest) map[string]*manifestv1alpha1.ConsumerServiceBindingManifest {
	byName := make(map[string]*manifestv1alpha1.ConsumerServiceBindingManifest, len(bindings))
	for _, binding := range bindings {
		byName[binding.GetBinding().GetConsumerName()] = binding
	}
	return byName
}

func consumerBindingContract(binding *manifestv1alpha1.ConsumerServiceBindingManifest) *manifestv1alpha1.ConsumerServiceBindingManifest {
	if binding == nil {
		return nil
	}
	clone := proto.Clone(binding).(*manifestv1alpha1.ConsumerServiceBindingManifest)
	clone.Location = nil
	if declared := clone.GetBinding(); declared != nil {
		sortConsumerEndpoints(declared.LocalEndpoints)
		sortMemberEndpointOverrides(declared.MemberEndpointOverrides)
		sortSubscriptions(declared.Subscriptions)
	}
	if selector := clone.GetProviderSelector(); selector != nil {
		sortProviderCandidates(selector.Candidates)
	}
	sortConsumerEndpoints(clone.LocalEndpoints)
	sortConsumerSubscriptions(clone.Subscriptions)
	sortProviderTPAnalyses(clone.InheritedMemberTpAnalysis)
	return clone
}

func sortMemberEndpointOverrides(overrides []*optionsv1alpha1.MemberEndpointOverride) {
	sort.Slice(overrides, func(i, j int) bool {
		if overrides[i].GetMemberId() != overrides[j].GetMemberId() {
			return overrides[i].GetMemberId() < overrides[j].GetMemberId()
		}
		return overrides[i].GetEndpointName() < overrides[j].GetEndpointName()
	})
}

func sortServiceDiscoveryOffer(offer *optionsv1alpha1.ServiceDiscoveryOffer) {
	if offer == nil {
		return
	}
	sort.Strings(offer.EndpointNames)
	sortDeliveryPolicies(offer.EventGroupDeliveryPolicies)
}

func sortDeliveryPolicies(policies []*optionsv1alpha1.EventGroupDeliveryPolicy) {
	sort.Slice(policies, func(i, j int) bool {
		left, right := policies[i], policies[j]
		if left.GetEventGroupName() != right.GetEventGroupName() {
			return left.GetEventGroupName() < right.GetEventGroupName()
		}
		if left.GetMode() != right.GetMode() {
			return left.GetMode() < right.GetMode()
		}
		if left.GetMulticastEndpoint().GetAddress() != right.GetMulticastEndpoint().GetAddress() {
			return left.GetMulticastEndpoint().GetAddress() < right.GetMulticastEndpoint().GetAddress()
		}
		return left.GetMulticastEndpoint().GetPort() < right.GetMulticastEndpoint().GetPort()
	})
}

func sortMemberTPConfigurations(configurations []*optionsv1alpha1.MemberTpConfiguration) {
	sort.Slice(configurations, func(i, j int) bool {
		if configurations[i].GetMemberId() != configurations[j].GetMemberId() {
			return configurations[i].GetMemberId() < configurations[j].GetMemberId()
		}
		return !configurations[i].GetTpWhenNeeded() && configurations[j].GetTpWhenNeeded()
	})
}

func sortConsumerEndpoints(endpoints []*optionsv1alpha1.ConsumerEndpoint) {
	sort.Slice(endpoints, func(i, j int) bool {
		left, right := endpoints[i], endpoints[j]
		if left.GetName() != right.GetName() {
			return left.GetName() < right.GetName()
		}
		if left.GetTransport() != right.GetTransport() {
			return left.GetTransport() < right.GetTransport()
		}
		if left.GetAddress() != right.GetAddress() {
			return left.GetAddress() < right.GetAddress()
		}
		return consumerEndpointPortKey(left) < consumerEndpointPortKey(right)
	})
}

func consumerEndpointPortKey(endpoint *optionsv1alpha1.ConsumerEndpoint) string {
	if endpoint == nil {
		return ""
	}
	switch port := endpoint.GetPort().(type) {
	case *optionsv1alpha1.ConsumerEndpoint_FixedPort:
		return fmt.Sprintf("fixed/%05d", port.FixedPort)
	case *optionsv1alpha1.ConsumerEndpoint_DynamicPortRange:
		return fmt.Sprintf("dynamic/%05d/%05d", port.DynamicPortRange.GetMinimumPort(), port.DynamicPortRange.GetMaximumPort())
	default:
		return "unspecified"
	}
}

func sortSubscriptions(subscriptions []*optionsv1alpha1.Subscription) {
	for _, subscription := range subscriptions {
		sort.Strings(subscription.LocalEndpointNames)
	}
	sort.Slice(subscriptions, func(i, j int) bool {
		left, right := subscriptions[i], subscriptions[j]
		if left.GetEventGroupName() != right.GetEventGroupName() {
			return left.GetEventGroupName() < right.GetEventGroupName()
		}
		if left.GetTtlSeconds() != right.GetTtlSeconds() {
			return left.GetTtlSeconds() < right.GetTtlSeconds()
		}
		if left.GetRequestInitialData() != right.GetRequestInitialData() {
			return !left.GetRequestInitialData() && right.GetRequestInitialData()
		}
		return strings.Join(left.GetLocalEndpointNames(), "\x00") < strings.Join(right.GetLocalEndpointNames(), "\x00")
	})
}

func sortProviderCandidates(candidates []*manifestv1alpha1.ProviderCandidateManifest) {
	sort.Slice(candidates, func(i, j int) bool {
		left, right := candidates[i], candidates[j]
		if left.GetProviderName() != right.GetProviderName() {
			return left.GetProviderName() < right.GetProviderName()
		}
		if left.GetServiceId() != right.GetServiceId() {
			return left.GetServiceId() < right.GetServiceId()
		}
		if left.GetInstanceId() != right.GetInstanceId() {
			return left.GetInstanceId() < right.GetInstanceId()
		}
		if left.GetService().GetFqn() != right.GetService().GetFqn() {
			return left.GetService().GetFqn() < right.GetService().GetFqn()
		}
		return left.GetInterfaceMajorVersion() < right.GetInterfaceMajorVersion()
	})
}

func sortConsumerSubscriptions(subscriptions []*manifestv1alpha1.ConsumerSubscriptionManifest) {
	for _, subscription := range subscriptions {
		if authored := subscription.GetSubscription(); authored != nil {
			sort.Strings(authored.LocalEndpointNames)
		}
	}
	sort.Slice(subscriptions, func(i, j int) bool {
		left, right := subscriptions[i].GetSubscription(), subscriptions[j].GetSubscription()
		if left.GetEventGroupName() != right.GetEventGroupName() {
			return left.GetEventGroupName() < right.GetEventGroupName()
		}
		if left.GetTtlSeconds() != right.GetTtlSeconds() {
			return left.GetTtlSeconds() < right.GetTtlSeconds()
		}
		return strings.Join(left.GetLocalEndpointNames(), "\x00") < strings.Join(right.GetLocalEndpointNames(), "\x00")
	})
}

func sortProviderTPAnalyses(analyses []*manifestv1alpha1.ProviderMemberTpAnalysisManifest) {
	for _, analysis := range analyses {
		sort.Slice(analysis.Directions, func(i, j int) bool {
			return analysis.Directions[i].GetDirection() < analysis.Directions[j].GetDirection()
		})
	}
	sort.Slice(analyses, func(i, j int) bool {
		return analyses[i].GetMemberId() < analyses[j].GetMemberId()
	})
}

func bindingsByKey(bindings []*manifestv1alpha1.ProviderServiceVersionBindingManifest) map[string]*manifestv1alpha1.ProviderServiceVersionBindingManifest {
	byKey := make(map[string]*manifestv1alpha1.ProviderServiceVersionBindingManifest, len(bindings))
	for _, binding := range bindings {
		lineage := binding.GetServiceLineage().GetId()
		major := binding.GetBinding().GetInterfaceMajorVersion()
		key := lineage + "/" + fmt.Sprint(major)
		byKey[key] = binding
	}
	return byKey
}

func (c *manifestComparison) classifyPayloadChanges() map[string]contractChange {
	baselinePayloads := payloadsByName(c.baseline.GetPayloadTypes())
	currentPayloads := payloadsByName(c.current.GetPayloadTypes())
	changes := make(map[string]contractChange, len(baselinePayloads))
	for name, baselinePayload := range baselinePayloads {
		currentPayload, found := currentPayloads[name]
		if !found {
			changes[name] = contractChange{
				level:    changeMajor,
				location: sourceLocation(baselinePayload),
				message:  fmt.Sprintf("payload %s was removed from the normalized contract", name),
			}
			continue
		}
		if proto.Equal(payloadContract(baselinePayload), payloadContract(currentPayload)) {
			changes[name] = contractChange{level: changeNone}
			continue
		}
		if payloadAppendOnly(baselinePayload, currentPayload) {
			changes[name] = contractChange{level: changeMinor}
			continue
		}
		changes[name] = contractChange{
			level:    changeMajor,
			location: sourceLocation(currentPayload),
			message:  fmt.Sprintf("payload %s changes an existing fixed or non-append-only layout", name),
		}
	}
	return changes
}

func payloadsByName(payloads []*manifestv1alpha1.PayloadTypeManifest) map[string]*manifestv1alpha1.PayloadTypeManifest {
	byName := make(map[string]*manifestv1alpha1.PayloadTypeManifest, len(payloads))
	for _, payload := range payloads {
		byName[payload.GetType().GetFqn()] = payload
	}
	return byName
}

func (c *manifestComparison) classifyEnumChanges() map[string]contractChange {
	baselineEnums := enumTypesByName(c.baseline.GetEnumTypes())
	currentEnums := enumTypesByName(c.current.GetEnumTypes())
	changes := make(map[string]contractChange, len(baselineEnums))
	for name, baselineEnum := range baselineEnums {
		currentEnum, found := currentEnums[name]
		if !found {
			changes[name] = contractChange{
				level:    changeMajor,
				location: sourceLocation(baselineEnum),
				message:  fmt.Sprintf("enum %s was removed from the normalized contract", name),
			}
			continue
		}
		if proto.Equal(enumContract(baselineEnum), enumContract(currentEnum)) {
			changes[name] = contractChange{level: changeNone}
			continue
		}
		changes[name] = contractChange{
			level:    changeMajor,
			location: sourceLocation(currentEnum),
			message:  fmt.Sprintf("enum %s changes its wire or value contract", name),
		}
	}
	return changes
}

func enumTypesByName(enums []*manifestv1alpha1.EnumManifest) map[string]*manifestv1alpha1.EnumManifest {
	byName := make(map[string]*manifestv1alpha1.EnumManifest, len(enums))
	for _, enum := range enums {
		byName[enum.GetType().GetFqn()] = enum
	}
	return byName
}

func enumContract(enum *manifestv1alpha1.EnumManifest) *manifestv1alpha1.EnumManifest {
	clone := proto.Clone(enum).(*manifestv1alpha1.EnumManifest)
	clone.Location = nil
	for _, value := range clone.GetValues() {
		value.Location = nil
	}
	clearEnumDocumentationURIs(clone)
	return clone
}

func clearEnumDocumentationURIs(enum *manifestv1alpha1.EnumManifest) {
	if enum == nil {
		return
	}
	if options := enum.GetOptions(); options != nil {
		options.DocumentationUris = nil
	}
	for _, value := range enum.GetValues() {
		if options := value.GetOptions(); options != nil {
			options.DocumentationUris = nil
		}
	}
}

func payloadContract(payload *manifestv1alpha1.PayloadTypeManifest) *manifestv1alpha1.PayloadTypeManifest {
	clone := proto.Clone(payload).(*manifestv1alpha1.PayloadTypeManifest)
	clone.Location = nil
	for _, element := range clone.GetLayoutElements() {
		clearElementLocations(element)
	}
	clearPayloadDocumentationURIs(clone)
	return clone
}

func clearPayloadDocumentationURIs(payload *manifestv1alpha1.PayloadTypeManifest) {
	if payload == nil {
		return
	}
	if options := payload.GetOptions(); options != nil {
		options.DocumentationUris = nil
	}
	for _, element := range payload.GetLayoutElements() {
		clearElementDocumentationURIs(element)
	}
}

func clearElementLocations(element *manifestv1alpha1.PayloadLayoutElement) {
	if member := element.GetMember(); member != nil {
		member.Location = nil
	}
	if union := element.GetUnion(); union != nil {
		union.Location = nil
		for _, alternative := range union.GetAlternatives() {
			alternative.Location = nil
		}
	}
}

func clearElementDocumentationURIs(element *manifestv1alpha1.PayloadLayoutElement) {
	if member := element.GetMember(); member != nil {
		if options := member.GetOptions(); options != nil {
			options.DocumentationUris = nil
		}
	}
	if union := element.GetUnion(); union != nil {
		if options := union.GetOptions(); options != nil {
			options.DocumentationUris = nil
		}
		for _, alternative := range union.GetAlternatives() {
			if options := alternative.GetOptions(); options != nil {
				options.DocumentationUris = nil
			}
		}
	}
}

func payloadAppendOnly(baseline, current *manifestv1alpha1.PayloadTypeManifest) bool {
	if baseline.GetEffectiveLayout() == nil || current.GetEffectiveLayout() == nil || !baseline.GetEffectiveLayout().GetExtensible() || !current.GetEffectiveLayout().GetExtensible() {
		return false
	}
	if baseline.GetEffectiveLayout().GetLengthFieldWidth() == optionsv1alpha1.LengthFieldWidth_LENGTH_FIELD_WIDTH_NONE || current.GetEffectiveLayout().GetLengthFieldWidth() != baseline.GetEffectiveLayout().GetLengthFieldWidth() || current.GetEffectiveLayout().GetByteOrder() != baseline.GetEffectiveLayout().GetByteOrder() {
		return false
	}
	if !proto.Equal(messageOptionsContract(baseline.GetOptions()), messageOptionsContract(current.GetOptions())) || len(current.GetLayoutElements()) <= len(baseline.GetLayoutElements()) {
		return false
	}
	for index, element := range baseline.GetLayoutElements() {
		if !proto.Equal(elementContract(element), elementContract(current.GetLayoutElements()[index])) {
			return false
		}
	}
	maximumOrder := baseline.GetEffectiveLayout().GetMaximumWireOrder()
	for _, element := range current.GetLayoutElements()[len(baseline.GetLayoutElements()):] {
		order := elementWireOrder(element)
		if order <= maximumOrder {
			return false
		}
		maximumOrder = order
	}
	return true
}

func elementContract(element *manifestv1alpha1.PayloadLayoutElement) *manifestv1alpha1.PayloadLayoutElement {
	clone := proto.Clone(element).(*manifestv1alpha1.PayloadLayoutElement)
	clearElementLocations(clone)
	clearElementDocumentationURIs(clone)
	return clone
}

func messageOptionsContract(options *optionsv1alpha1.MessageOptionsSpec) *optionsv1alpha1.MessageOptionsSpec {
	if options == nil {
		return nil
	}
	clone := proto.Clone(options).(*optionsv1alpha1.MessageOptionsSpec)
	clone.DocumentationUris = nil
	return clone
}

func elementWireOrder(element *manifestv1alpha1.PayloadLayoutElement) uint32 {
	if member := element.GetMember(); member != nil {
		return member.GetOptions().GetWireOrder()
	}
	if union := element.GetUnion(); union != nil {
		return union.GetOptions().GetWireOrder()
	}
	return 0
}

func (c *manifestComparison) compareServices(payloadChanges, enumChanges map[string]contractChange) {
	baselineServices := servicesByKey(c.baseline.GetServices())
	currentServices := servicesByKey(c.current.GetServices())
	currentByLineage := servicesByLineage(c.current.GetServices())
	baselinePayloads := payloadsByName(c.baseline.GetPayloadTypes())
	baselineEnums := enumTypesByName(c.baseline.GetEnumTypes())

	for key, baselineService := range baselineServices {
		currentService, found := currentServices[key]
		if !found {
			if !hasHigherMajor(currentByLineage[baselineService.GetServiceLineage().GetId()], baselineService.GetInterfaceMajorVersion()) {
				c.diagnostics = append(c.diagnostics, compatibilityDiagnostic(sourceLocation(baselineService), "service lineage %q major %d was removed without a higher interface major", baselineService.GetServiceLineage().GetId(), baselineService.GetInterfaceMajorVersion()))
			}
			c.profileNeed = maximumChange(c.profileNeed, changeMajor)
			continue
		}

		level := classifyServiceChange(baselineService, currentService)
		for payloadName := range reachablePayloads(baselineService, baselinePayloads) {
			change := payloadChanges[payloadName]
			level = maximumChange(level, change.level)
			c.reportPayloadChange(payloadName, change)
		}
		for enumName := range reachableEnumTypes(baselineService, baselinePayloads, baselineEnums) {
			change := enumChanges[enumName]
			level = maximumChange(level, change.level)
			c.reportEnumChange(enumName, change)
		}
		c.requireInterfaceVersion(baselineService, currentService, level)
		c.profileNeed = maximumChange(c.profileNeed, level)
	}

	for key, currentService := range currentServices {
		if _, found := baselineServices[key]; !found {
			if baselineLineage, found := servicesByLineage(c.baseline.GetServices())[currentService.GetServiceLineage().GetId()]; found && len(baselineLineage) > 0 {
				c.profileNeed = maximumChange(c.profileNeed, changeMajor)
			} else {
				c.profileNeed = maximumChange(c.profileNeed, changeMinor)
			}
		}
	}
}

func (c *manifestComparison) reportPayloadChange(name string, change contractChange) {
	if change.level != changeMajor || change.message == "" {
		return
	}
	if _, reported := c.reportedPayloadChanges[name]; reported {
		return
	}
	c.reportedPayloadChanges[name] = struct{}{}
	c.diagnostics = append(c.diagnostics, staticEvolutionDiagnostic(change.location, "%s", change.message))
}

func (c *manifestComparison) reportEnumChange(name string, change contractChange) {
	if change.level != changeMajor || change.message == "" {
		return
	}
	if _, reported := c.reportedEnumChanges[name]; reported {
		return
	}
	c.reportedEnumChanges[name] = struct{}{}
	c.diagnostics = append(c.diagnostics, staticEvolutionDiagnostic(change.location, "%s", change.message))
}

func reachableEnumTypes(
	service *manifestv1alpha1.ServiceManifest,
	payloads map[string]*manifestv1alpha1.PayloadTypeManifest,
	enums map[string]*manifestv1alpha1.EnumManifest,
) map[string]struct{} {
	reachable := make(map[string]struct{})
	for payloadName := range reachablePayloads(service, payloads) {
		payload := payloads[payloadName]
		for _, element := range payload.GetLayoutElements() {
			if member := element.GetMember(); member != nil {
				recordReachableEnum(reachable, enums, member.GetType().GetFqn())
			}
			if union := element.GetUnion(); union != nil {
				for _, alternative := range union.GetAlternatives() {
					recordReachableEnum(reachable, enums, alternative.GetType().GetFqn())
				}
			}
		}
	}
	return reachable
}

func recordReachableEnum(reachable map[string]struct{}, enums map[string]*manifestv1alpha1.EnumManifest, name string) {
	if _, found := enums[name]; found {
		reachable[name] = struct{}{}
	}
}

func servicesByKey(services []*manifestv1alpha1.ServiceManifest) map[string]*manifestv1alpha1.ServiceManifest {
	byKey := make(map[string]*manifestv1alpha1.ServiceManifest, len(services))
	for _, service := range services {
		byKey[serviceKey(service)] = service
	}
	return byKey
}

func servicesByLineage(services []*manifestv1alpha1.ServiceManifest) map[string][]*manifestv1alpha1.ServiceManifest {
	byLineage := make(map[string][]*manifestv1alpha1.ServiceManifest)
	for _, service := range services {
		lineage := service.GetServiceLineage().GetId()
		byLineage[lineage] = append(byLineage[lineage], service)
	}
	return byLineage
}

func serviceKey(service *manifestv1alpha1.ServiceManifest) string {
	return service.GetServiceLineage().GetId() + "/" + fmt.Sprint(service.GetInterfaceMajorVersion())
}

func hasHigherMajor(services []*manifestv1alpha1.ServiceManifest, major uint32) bool {
	for _, service := range services {
		if service.GetInterfaceMajorVersion() > major {
			return true
		}
	}
	return false
}

func classifyServiceChange(baseline, current *manifestv1alpha1.ServiceManifest) changeLevel {
	if baseline.GetService().GetFqn() != current.GetService().GetFqn() || baseline.GetServiceLineage().GetId() != current.GetServiceLineage().GetId() {
		return changeMajor
	}
	level := classifyEventGroups(baseline.GetEventGroups(), current.GetEventGroups())
	level = maximumChange(level, classifyServiceFields(baseline.GetFields(), current.GetFields()))
	if current.GetInterfaceMinorVersion() > baseline.GetInterfaceMinorVersion() {
		level = maximumChange(level, changeMinor)
	}
	baselineMembers := membersByID(baseline.GetMembers())
	currentMembers := membersByID(current.GetMembers())
	for identifier, baselineMember := range baselineMembers {
		currentMember, found := currentMembers[identifier]
		if !found || !sameMemberContract(baselineMember, currentMember) {
			return changeMajor
		}
	}
	for identifier := range currentMembers {
		if _, found := baselineMembers[identifier]; !found {
			level = maximumChange(level, changeMinor)
		}
	}
	return level
}

func classifyServiceFields(baseline, current []*manifestv1alpha1.ServiceFieldManifest) changeLevel {
	baselineFields := serviceFieldsByName(baseline)
	currentFields := serviceFieldsByName(current)
	level := changeNone
	for name, baselineField := range baselineFields {
		currentField, found := currentFields[name]
		if !found || !sameServiceFieldContract(baselineField.GetField(), currentField.GetField()) {
			return changeMajor
		}
	}
	for name := range currentFields {
		if _, found := baselineFields[name]; !found {
			level = maximumChange(level, changeMinor)
		}
	}
	return level
}

func serviceFieldsByName(fields []*manifestv1alpha1.ServiceFieldManifest) map[string]*manifestv1alpha1.ServiceFieldManifest {
	byName := make(map[string]*manifestv1alpha1.ServiceFieldManifest, len(fields))
	for _, field := range fields {
		byName[field.GetField().GetName()] = field
	}
	return byName
}

func sameServiceFieldContract(left, right *optionsv1alpha1.ServiceField) bool {
	if left == nil || right == nil {
		return left == right
	}
	return left.GetValueType().GetFqn() == right.GetValueType().GetFqn() &&
		left.GetGetterMethod().GetFqn() == right.GetGetterMethod().GetFqn() &&
		left.GetSetterMethod().GetFqn() == right.GetSetterMethod().GetFqn() &&
		left.GetNotifierId() == right.GetNotifierId() &&
		sameStringSet(left.GetEventGroupNames(), right.GetEventGroupNames()) &&
		proto.Equal(left.GetPublication(), right.GetPublication())
}

func classifyEventGroups(baseline, current []*optionsv1alpha1.EventGroup) changeLevel {
	baselineByName := make(map[string]*optionsv1alpha1.EventGroup, len(baseline))
	for _, group := range baseline {
		baselineByName[group.GetName()] = group
	}
	currentByName := make(map[string]*optionsv1alpha1.EventGroup, len(current))
	for _, group := range current {
		currentByName[group.GetName()] = group
	}
	level := changeNone
	for name, baselineGroup := range baselineByName {
		currentGroup, found := currentByName[name]
		if !found || baselineGroup.GetId() != currentGroup.GetId() {
			return changeMajor
		}
	}
	for name := range currentByName {
		if _, found := baselineByName[name]; !found {
			level = maximumChange(level, changeMinor)
		}
	}
	return level
}

func membersByID(members []*manifestv1alpha1.ServiceMemberManifest) map[uint32]*manifestv1alpha1.ServiceMemberManifest {
	byID := make(map[uint32]*manifestv1alpha1.ServiceMemberManifest, len(members))
	for _, member := range members {
		byID[member.GetMemberId()] = member
	}
	return byID
}

func sameMemberContract(left, right *manifestv1alpha1.ServiceMemberManifest) bool {
	if left.GetName() != right.GetName() || left.GetKind() != right.GetKind() || left.GetRequestPayloadType().GetFqn() != right.GetRequestPayloadType().GetFqn() || left.GetResponsePayloadType().GetFqn() != right.GetResponsePayloadType().GetFqn() {
		return false
	}
	if !sameStringSet(left.GetEventGroupNames(), right.GetEventGroupNames()) {
		return false
	}
	return proto.Equal(left.GetPublication(), right.GetPublication()) && sameMethodContract(left.GetMethodOptions(), right.GetMethodOptions())
}

func sameStringSet(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	leftCopy := append([]string(nil), left...)
	rightCopy := append([]string(nil), right...)
	sort.Strings(leftCopy)
	sort.Strings(rightCopy)
	for index := range leftCopy {
		if leftCopy[index] != rightCopy[index] {
			return false
		}
	}
	return true
}

func reachablePayloads(service *manifestv1alpha1.ServiceManifest, payloads map[string]*manifestv1alpha1.PayloadTypeManifest) map[string]struct{} {
	reachable := make(map[string]struct{})
	pending := make([]string, 0)
	for _, member := range service.GetMembers() {
		pending = append(pending, member.GetRequestPayloadType().GetFqn(), member.GetResponsePayloadType().GetFqn())
		pending = append(pending, member.GetMethodOptions().GetExceptionPayloadType().GetFqn())
	}
	for len(pending) > 0 {
		name := pending[0]
		pending = pending[1:]
		if name == "" || name == ".google.protobuf.Empty" {
			continue
		}
		if _, found := reachable[name]; found {
			continue
		}
		payload, found := payloads[name]
		if !found {
			continue
		}
		reachable[name] = struct{}{}
		for _, element := range payload.GetLayoutElements() {
			if member := element.GetMember(); member != nil {
				pending = append(pending, member.GetType().GetFqn())
			}
			if union := element.GetUnion(); union != nil {
				for _, alternative := range union.GetAlternatives() {
					pending = append(pending, alternative.GetType().GetFqn())
				}
			}
		}
	}
	return reachable
}

func (c *manifestComparison) requireInterfaceVersion(
	baseline *manifestv1alpha1.ServiceManifest,
	current *manifestv1alpha1.ServiceManifest,
	required changeLevel,
) {
	if current.GetInterfaceMajorVersion() < baseline.GetInterfaceMajorVersion() || (current.GetInterfaceMajorVersion() == baseline.GetInterfaceMajorVersion() && current.GetInterfaceMinorVersion() < baseline.GetInterfaceMinorVersion()) {
		c.diagnostics = append(c.diagnostics, compatibilityDiagnostic(sourceLocation(current), "service %s interface version regressed from %d.%d to %d.%d", current.GetService().GetFqn(), baseline.GetInterfaceMajorVersion(), baseline.GetInterfaceMinorVersion(), current.GetInterfaceMajorVersion(), current.GetInterfaceMinorVersion()))
		return
	}
	switch required {
	case changeMajor:
		c.diagnostics = append(c.diagnostics, compatibilityDiagnostic(sourceLocation(current), "service %s changes its resolved contract without an interface-major transition", current.GetService().GetFqn()))
	case changeMinor:
		if current.GetInterfaceMinorVersion() <= baseline.GetInterfaceMinorVersion() {
			c.diagnostics = append(c.diagnostics, compatibilityDiagnostic(sourceLocation(current), "service %s adds to its resolved contract without an interface-minor transition", current.GetService().GetFqn()))
		}
	}
}

func (c *manifestComparison) requireProfileVersion(required changeLevel) {
	baseline := c.baseline.GetProfile().GetVersion()
	current := c.current.GetProfile().GetVersion()
	if baseline == nil || current == nil {
		c.diagnostics = append(c.diagnostics, compatibilityDiagnostic(sourceLocation(c.current.GetProfile()), "baseline and current profiles must declare semantic versions for compatibility comparison"))
		return
	}
	if compareVersion(current, baseline) < 0 {
		c.diagnostics = append(c.diagnostics, compatibilityDiagnostic(sourceLocation(c.current.GetProfile()), "profile version regressed from %s to %s", versionText(baseline), versionText(current)))
		return
	}
	switch required {
	case changePatch:
		if compareVersion(current, baseline) <= 0 {
			c.diagnostics = append(c.diagnostics, compatibilityDiagnostic(sourceLocation(c.current.GetProfile()), "profile metadata changed without a profile patch transition from %s", versionText(baseline)))
		}
	case changeMinor:
		if current.GetMajor() == baseline.GetMajor() && current.GetMinor() <= baseline.GetMinor() {
			c.diagnostics = append(c.diagnostics, compatibilityDiagnostic(sourceLocation(c.current.GetProfile()), "profile has compatible additions without a profile-minor transition from %s", versionText(baseline)))
		}
	case changeMajor:
		if current.GetMajor() <= baseline.GetMajor() {
			c.diagnostics = append(c.diagnostics, compatibilityDiagnostic(sourceLocation(c.current.GetProfile()), "profile changes deployment or interface-major contract without a profile-major transition from %s", versionText(baseline)))
		}
	}
}

func compareVersion(left, right *optionsv1alpha1.SemanticVersion) int {
	if left.GetMajor() != right.GetMajor() {
		if left.GetMajor() < right.GetMajor() {
			return -1
		}
		return 1
	}
	if left.GetMinor() != right.GetMinor() {
		if left.GetMinor() < right.GetMinor() {
			return -1
		}
		return 1
	}
	if left.GetPatch() != right.GetPatch() {
		if left.GetPatch() < right.GetPatch() {
			return -1
		}
		return 1
	}
	return comparePrerelease(left.GetPrerelease(), right.GetPrerelease())
}

func comparePrerelease(left, right string) int {
	if left == right {
		return 0
	}
	if left == "" {
		return 1
	}
	if right == "" {
		return -1
	}
	leftIdentifiers := strings.Split(left, ".")
	rightIdentifiers := strings.Split(right, ".")
	for index := 0; index < len(leftIdentifiers) && index < len(rightIdentifiers); index++ {
		comparison := comparePrereleaseIdentifier(leftIdentifiers[index], rightIdentifiers[index])
		if comparison != 0 {
			return comparison
		}
	}
	if len(leftIdentifiers) < len(rightIdentifiers) {
		return -1
	}
	if len(leftIdentifiers) > len(rightIdentifiers) {
		return 1
	}
	return 0
}

func comparePrereleaseIdentifier(left, right string) int {
	leftNumeric := isDecimalIdentifier(left)
	rightNumeric := isDecimalIdentifier(right)
	if leftNumeric && rightNumeric {
		if len(left) != len(right) {
			if len(left) < len(right) {
				return -1
			}
			return 1
		}
		return strings.Compare(left, right)
	}
	if leftNumeric != rightNumeric {
		if leftNumeric {
			return -1
		}
		return 1
	}
	return strings.Compare(left, right)
}

func isDecimalIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func versionText(version *optionsv1alpha1.SemanticVersion) string {
	if version == nil {
		return "(unspecified)"
	}
	value := fmt.Sprintf("%d.%d.%d", version.GetMajor(), version.GetMinor(), version.GetPatch())
	if version.GetPrerelease() != "" {
		value += "-" + version.GetPrerelease()
	}
	if version.GetBuildMetadata() != "" {
		value += "+" + version.GetBuildMetadata()
	}
	return value
}

func sourceLocation(value interface {
	GetLocation() *manifestv1alpha1.SourceLocation
}) *manifestv1alpha1.SourceLocation {
	if value == nil {
		return nil
	}
	return value.GetLocation()
}
