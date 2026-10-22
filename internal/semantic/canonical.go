package semantic

import (
	"sort"
	"strings"

	optionsv1alpha1 "github.com/eshenhu/someip_proto/gen/go/io/github/someip_proto/options/v1alpha1"
	"google.golang.org/protobuf/proto"
)

func canonicalDocumentationURIs(values []string) []string {
	return canonicalStrings(values)
}

func canonicalStrings(values []string) []string {
	canonical := append([]string(nil), values...)
	sort.Strings(canonical)
	return canonical
}

func canonicalProviderInstance(value *optionsv1alpha1.ProviderInstance) *optionsv1alpha1.ProviderInstance {
	if value == nil {
		return nil
	}
	canonical := proto.Clone(value).(*optionsv1alpha1.ProviderInstance)
	canonical.MemberEndpointOverrides = canonicalMemberEndpointOverrides(canonical.GetMemberEndpointOverrides())
	for index, binding := range canonical.GetServiceVersionBindings() {
		canonical.ServiceVersionBindings[index] = canonicalServiceVersionBinding(binding)
	}
	sort.Slice(canonical.Endpoints, func(i, j int) bool {
		left, right := canonical.Endpoints[i], canonical.Endpoints[j]
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
	sort.Slice(canonical.ServiceVersionBindings, func(i, j int) bool {
		left, right := canonical.ServiceVersionBindings[i], canonical.ServiceVersionBindings[j]
		if left.GetInterfaceMajorVersion() != right.GetInterfaceMajorVersion() {
			return left.GetInterfaceMajorVersion() < right.GetInterfaceMajorVersion()
		}
		return left.GetService().GetFqn() < right.GetService().GetFqn()
	})
	return canonical
}

func canonicalServiceVersionBinding(value *optionsv1alpha1.ServiceVersionBinding) *optionsv1alpha1.ServiceVersionBinding {
	if value == nil {
		return nil
	}
	canonical := proto.Clone(value).(*optionsv1alpha1.ServiceVersionBinding)
	canonical.MemberEndpointOverrides = canonicalMemberEndpointOverrides(canonical.GetMemberEndpointOverrides())
	canonical.ServiceDiscoveryOffer = canonicalServiceDiscoveryOffer(canonical.GetServiceDiscoveryOffer())
	canonical.MemberTpConfigurations = canonicalMemberTPConfigurations(canonical.GetMemberTpConfigurations())
	return canonical
}

func canonicalServiceDiscoveryOffer(value *optionsv1alpha1.ServiceDiscoveryOffer) *optionsv1alpha1.ServiceDiscoveryOffer {
	if value == nil {
		return nil
	}
	canonical := proto.Clone(value).(*optionsv1alpha1.ServiceDiscoveryOffer)
	canonical.EndpointNames = canonicalStrings(canonical.GetEndpointNames())
	for index, policy := range canonical.GetEventGroupDeliveryPolicies() {
		canonical.EventGroupDeliveryPolicies[index] = canonicalEventGroupDeliveryPolicy(policy)
	}
	sort.Slice(canonical.EventGroupDeliveryPolicies, func(i, j int) bool {
		left, right := canonical.EventGroupDeliveryPolicies[i], canonical.EventGroupDeliveryPolicies[j]
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
	return canonical
}

func canonicalEventGroupDeliveryPolicy(value *optionsv1alpha1.EventGroupDeliveryPolicy) *optionsv1alpha1.EventGroupDeliveryPolicy {
	if value == nil {
		return nil
	}
	return proto.Clone(value).(*optionsv1alpha1.EventGroupDeliveryPolicy)
}

func canonicalMemberTPConfigurations(values []*optionsv1alpha1.MemberTpConfiguration) []*optionsv1alpha1.MemberTpConfiguration {
	canonical := make([]*optionsv1alpha1.MemberTpConfiguration, 0, len(values))
	for _, value := range values {
		if value == nil {
			canonical = append(canonical, nil)
			continue
		}
		canonical = append(canonical, proto.Clone(value).(*optionsv1alpha1.MemberTpConfiguration))
	}
	sort.Slice(canonical, func(i, j int) bool {
		if canonical[i].GetMemberId() != canonical[j].GetMemberId() {
			return canonical[i].GetMemberId() < canonical[j].GetMemberId()
		}
		return !canonical[i].GetTpWhenNeeded() && canonical[j].GetTpWhenNeeded()
	})
	return canonical
}

func canonicalConsumerServiceBinding(value *optionsv1alpha1.ConsumerServiceBinding) *optionsv1alpha1.ConsumerServiceBinding {
	if value == nil {
		return nil
	}
	canonical := proto.Clone(value).(*optionsv1alpha1.ConsumerServiceBinding)
	canonical.MemberEndpointOverrides = canonicalMemberEndpointOverrides(canonical.GetMemberEndpointOverrides())
	canonical.LocalEndpoints = canonicalConsumerEndpoints(canonical.GetLocalEndpoints())
	canonical.Subscriptions = canonicalSubscriptions(canonical.GetSubscriptions())
	return canonical
}

func canonicalConsumerEndpoints(values []*optionsv1alpha1.ConsumerEndpoint) []*optionsv1alpha1.ConsumerEndpoint {
	canonical := make([]*optionsv1alpha1.ConsumerEndpoint, 0, len(values))
	for _, value := range values {
		if value == nil {
			canonical = append(canonical, nil)
			continue
		}
		canonical = append(canonical, proto.Clone(value).(*optionsv1alpha1.ConsumerEndpoint))
	}
	sort.Slice(canonical, func(i, j int) bool {
		left, right := canonical[i], canonical[j]
		if left.GetName() != right.GetName() {
			return left.GetName() < right.GetName()
		}
		if left.GetTransport() != right.GetTransport() {
			return left.GetTransport() < right.GetTransport()
		}
		if left.GetAddress() != right.GetAddress() {
			return left.GetAddress() < right.GetAddress()
		}
		if left.GetFixedPort() != right.GetFixedPort() {
			return left.GetFixedPort() < right.GetFixedPort()
		}
		leftRange, rightRange := left.GetDynamicPortRange(), right.GetDynamicPortRange()
		if leftRange.GetMinimumPort() != rightRange.GetMinimumPort() {
			return leftRange.GetMinimumPort() < rightRange.GetMinimumPort()
		}
		return leftRange.GetMaximumPort() < rightRange.GetMaximumPort()
	})
	return canonical
}

func canonicalSubscriptions(values []*optionsv1alpha1.Subscription) []*optionsv1alpha1.Subscription {
	canonical := make([]*optionsv1alpha1.Subscription, 0, len(values))
	for _, value := range values {
		if value == nil {
			canonical = append(canonical, nil)
			continue
		}
		cloned := proto.Clone(value).(*optionsv1alpha1.Subscription)
		cloned.LocalEndpointNames = canonicalStrings(cloned.GetLocalEndpointNames())
		canonical = append(canonical, cloned)
	}
	sort.Slice(canonical, func(i, j int) bool {
		left, right := canonical[i], canonical[j]
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
	return canonical
}

func canonicalMemberEndpointOverrides(values []*optionsv1alpha1.MemberEndpointOverride) []*optionsv1alpha1.MemberEndpointOverride {
	canonical := make([]*optionsv1alpha1.MemberEndpointOverride, 0, len(values))
	for _, value := range values {
		if value == nil {
			canonical = append(canonical, nil)
			continue
		}
		canonical = append(canonical, proto.Clone(value).(*optionsv1alpha1.MemberEndpointOverride))
	}
	sort.Slice(canonical, func(i, j int) bool {
		if canonical[i].GetMemberId() != canonical[j].GetMemberId() {
			return canonical[i].GetMemberId() < canonical[j].GetMemberId()
		}
		return canonical[i].GetEndpointName() < canonical[j].GetEndpointName()
	})
	return canonical
}

func canonicalMethodOptions(value *optionsv1alpha1.MethodOptionsSpec) *optionsv1alpha1.MethodOptionsSpec {
	if value == nil {
		return nil
	}
	canonical := proto.Clone(value).(*optionsv1alpha1.MethodOptionsSpec)
	canonical.DocumentationUris = canonicalDocumentationURIs(canonical.GetDocumentationUris())
	sort.Slice(canonical.ReturnCodes, func(i, j int) bool {
		left, right := canonical.ReturnCodes[i], canonical.ReturnCodes[j]
		if left.GetCode() != right.GetCode() {
			return left.GetCode() < right.GetCode()
		}
		if left.GetName() != right.GetName() {
			return left.GetName() < right.GetName()
		}
		return left.GetDescription() < right.GetDescription()
	})
	return canonical
}

func canonicalServiceOptions(value *optionsv1alpha1.ServiceOptionsSpec) *optionsv1alpha1.ServiceOptionsSpec {
	if value == nil {
		return nil
	}
	canonical := proto.Clone(value).(*optionsv1alpha1.ServiceOptionsSpec)
	canonical.DocumentationUris = canonicalDocumentationURIs(canonical.GetDocumentationUris())
	canonical.EventGroups = canonicalEventGroups(canonical.GetEventGroups())
	for index, event := range canonical.GetEvents() {
		canonical.Events[index] = canonicalEvent(event)
	}
	for index, field := range canonical.GetFields() {
		canonical.Fields[index] = canonicalServiceField(field)
	}
	sort.Slice(canonical.Events, func(i, j int) bool {
		left, right := canonical.Events[i], canonical.Events[j]
		if left.GetId() != right.GetId() {
			return left.GetId() < right.GetId()
		}
		return left.GetName() < right.GetName()
	})
	sort.Slice(canonical.Fields, func(i, j int) bool {
		return canonical.Fields[i].GetName() < canonical.Fields[j].GetName()
	})
	return canonical
}

func canonicalEvent(value *optionsv1alpha1.Event) *optionsv1alpha1.Event {
	if value == nil {
		return nil
	}
	canonical := proto.Clone(value).(*optionsv1alpha1.Event)
	canonical.EventGroupNames = canonicalStrings(canonical.GetEventGroupNames())
	return canonical
}

func canonicalServiceField(value *optionsv1alpha1.ServiceField) *optionsv1alpha1.ServiceField {
	if value == nil {
		return nil
	}
	canonical := proto.Clone(value).(*optionsv1alpha1.ServiceField)
	canonical.EventGroupNames = canonicalStrings(canonical.GetEventGroupNames())
	return canonical
}

func canonicalEventGroups(values []*optionsv1alpha1.EventGroup) []*optionsv1alpha1.EventGroup {
	canonical := make([]*optionsv1alpha1.EventGroup, 0, len(values))
	for _, value := range values {
		if value == nil {
			canonical = append(canonical, nil)
			continue
		}
		canonical = append(canonical, proto.Clone(value).(*optionsv1alpha1.EventGroup))
	}
	sort.Slice(canonical, func(i, j int) bool {
		if canonical[i].GetId() != canonical[j].GetId() {
			return canonical[i].GetId() < canonical[j].GetId()
		}
		return canonical[i].GetName() < canonical[j].GetName()
	})
	return canonical
}

func canonicalMessageOptions(value *optionsv1alpha1.MessageOptionsSpec) *optionsv1alpha1.MessageOptionsSpec {
	if value == nil {
		return nil
	}
	canonical := proto.Clone(value).(*optionsv1alpha1.MessageOptionsSpec)
	canonical.DocumentationUris = canonicalDocumentationURIs(canonical.GetDocumentationUris())
	if layout := canonical.GetStructLayout(); layout != nil {
		sort.Slice(layout.Padding, func(i, j int) bool {
			left, right := layout.Padding[i], layout.Padding[j]
			leftKind, leftOrder := canonicalPaddingPosition(left)
			rightKind, rightOrder := canonicalPaddingPosition(right)
			if leftKind != rightKind {
				return leftKind < rightKind
			}
			if leftOrder != rightOrder {
				return leftOrder < rightOrder
			}
			return left.GetByteCount() < right.GetByteCount()
		})
	}
	return canonical
}

func canonicalPaddingPosition(value *optionsv1alpha1.Padding) (uint32, uint32) {
	if value == nil {
		return 0, 0
	}
	switch position := value.GetPosition().(type) {
	case *optionsv1alpha1.Padding_BeforeWireOrder:
		return 1, position.BeforeWireOrder
	case *optionsv1alpha1.Padding_AtEnd:
		return 2, 0
	default:
		return 0, 0
	}
}

func canonicalFieldOptions(value *optionsv1alpha1.FieldOptionsSpec) *optionsv1alpha1.FieldOptionsSpec {
	if value == nil {
		return nil
	}
	canonical := proto.Clone(value).(*optionsv1alpha1.FieldOptionsSpec)
	canonical.DocumentationUris = canonicalDocumentationURIs(canonical.GetDocumentationUris())
	if bitfield := canonical.GetBitfield(); bitfield != nil {
		sort.Slice(bitfield.NamedBits, func(i, j int) bool {
			left, right := bitfield.NamedBits[i], bitfield.NamedBits[j]
			if left.GetBitIndex() != right.GetBitIndex() {
				return left.GetBitIndex() < right.GetBitIndex()
			}
			if left.GetName() != right.GetName() {
				return left.GetName() < right.GetName()
			}
			if left.GetZeroName() != right.GetZeroName() {
				return left.GetZeroName() < right.GetZeroName()
			}
			return left.GetOneName() < right.GetOneName()
		})
	}
	return canonical
}

func canonicalOneofOptions(value *optionsv1alpha1.OneofOptionsSpec) *optionsv1alpha1.OneofOptionsSpec {
	if value == nil {
		return nil
	}
	canonical := proto.Clone(value).(*optionsv1alpha1.OneofOptionsSpec)
	canonical.DocumentationUris = canonicalDocumentationURIs(canonical.GetDocumentationUris())
	return canonical
}

func canonicalEnumOptions(value *optionsv1alpha1.EnumOptionsSpec) *optionsv1alpha1.EnumOptionsSpec {
	if value == nil {
		return nil
	}
	canonical := proto.Clone(value).(*optionsv1alpha1.EnumOptionsSpec)
	canonical.DocumentationUris = canonicalDocumentationURIs(canonical.GetDocumentationUris())
	return canonical
}

func canonicalEnumValueOptions(value *optionsv1alpha1.EnumValueOptionsSpec) *optionsv1alpha1.EnumValueOptionsSpec {
	if value == nil {
		return nil
	}
	canonical := proto.Clone(value).(*optionsv1alpha1.EnumValueOptionsSpec)
	canonical.DocumentationUris = canonicalDocumentationURIs(canonical.GetDocumentationUris())
	return canonical
}
