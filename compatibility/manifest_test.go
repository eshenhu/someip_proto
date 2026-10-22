package compatibility

import (
	"testing"

	manifestv1alpha1 "github.com/eshenhu/someip_proto/gen/go/io/github/someip_proto/manifest/v1alpha1"
	optionsv1alpha1 "github.com/eshenhu/someip_proto/gen/go/io/github/someip_proto/options/v1alpha1"
	"google.golang.org/protobuf/proto"
)

func TestCompareManifests_identicalContract(t *testing.T) {
	manifest := comparisonManifest(false)

	if diagnostics := CompareManifests(manifest, proto.Clone(manifest).(*manifestv1alpha1.Manifest)); len(diagnostics) != 0 {
		t.Fatalf("CompareManifests() diagnostics = %#v, want none", diagnostics)
	}
}

func TestCompareManifests_fixedPayloadMutationRequiresMajorTransition(t *testing.T) {
	baseline := comparisonManifest(false)
	current := proto.Clone(baseline).(*manifestv1alpha1.Manifest)
	current.GetPayloadTypes()[0].GetLayoutElements()[0].GetMember().GetOptions().WireOrder = 2
	current.GetPayloadTypes()[0].GetEffectiveLayout().MaximumWireOrder = 2

	diagnostics := CompareManifests(baseline, current)
	if !hasRule(diagnostics, "SIP-COMPAT-002") {
		t.Fatalf("diagnostics = %#v, want static-evolution diagnostic", diagnostics)
	}
	if !hasRule(diagnostics, "SIP-COMPAT-001") {
		t.Fatalf("diagnostics = %#v, want required-major-transition diagnostic", diagnostics)
	}
}

func TestCompareManifests_majorTransitionCanReplacePayloadContract(t *testing.T) {
	baseline := comparisonManifest(false)
	current := proto.Clone(baseline).(*manifestv1alpha1.Manifest)
	current.GetProfile().GetVersion().Major = 2
	service := current.GetServices()[0]
	service.InterfaceMajorVersion = 2
	service.Service.Fqn = ".example.ServiceV2"
	service.GetMembers()[0].RequestPayloadType.Fqn = ".example.PayloadV2"
	current.GetPayloadTypes()[0].Type.Fqn = ".example.PayloadV2"

	if diagnostics := CompareManifests(baseline, current); len(diagnostics) != 0 {
		t.Fatalf("CompareManifests() diagnostics = %#v, want accepted interface/profile major transition", diagnostics)
	}
}

func TestCompareManifests_extensibleAppendWithMinorTransitions(t *testing.T) {
	baseline := comparisonManifest(true)
	current := proto.Clone(baseline).(*manifestv1alpha1.Manifest)
	current.GetProfile().Version.Minor = 1
	current.GetServices()[0].InterfaceMinorVersion = 1
	payload := current.GetPayloadTypes()[0]
	payload.LayoutElements = append(payload.LayoutElements, payloadMemberElement("second", 2, 2))
	payload.EffectiveLayout.MaximumWireOrder = 2
	payload.EncodedSize = &manifestv1alpha1.EncodedSize{ExactBytes: 2, MinimumBytes: 2, MaximumBytes: 2}

	if diagnostics := CompareManifests(baseline, current); len(diagnostics) != 0 {
		t.Fatalf("CompareManifests() diagnostics = %#v, want none", diagnostics)
	}
}

func TestCompareManifests_interfaceMinorRequiresProfileMinor(t *testing.T) {
	baseline := comparisonManifest(false)
	current := proto.Clone(baseline).(*manifestv1alpha1.Manifest)
	current.GetServices()[0].InterfaceMinorVersion = 1

	if diagnostics := CompareManifests(baseline, current); !hasRule(diagnostics, "SIP-COMPAT-001") {
		t.Fatalf("CompareManifests() diagnostics = %#v, want profile-minor transition error", diagnostics)
	}
	current.GetProfile().GetVersion().Minor = 1
	if diagnostics := CompareManifests(baseline, current); len(diagnostics) != 0 {
		t.Fatalf("CompareManifests() diagnostics = %#v, want accepted matching profile-minor transition", diagnostics)
	}
}

func TestCompareManifests_methodBehaviorRequiresMajorTransition(t *testing.T) {
	baseline := comparisonManifest(false)
	member := baseline.GetServices()[0].GetMembers()[0]
	member.MethodOptions = &optionsv1alpha1.MethodOptionsSpec{
		MemberId:       member.GetMemberId(),
		Kind:           optionsv1alpha1.MethodKind_METHOD_KIND_REQUEST_RESPONSE,
		ResponsePolicy: optionsv1alpha1.ResponsePolicy_RESPONSE_POLICY_RESPONSE_ONLY,
	}
	current := proto.Clone(baseline).(*manifestv1alpha1.Manifest)
	current.GetServices()[0].GetMembers()[0].GetMethodOptions().ResponsePolicy = optionsv1alpha1.ResponsePolicy_RESPONSE_POLICY_EXCEPTION_ONLY

	if diagnostics := CompareManifests(baseline, current); !hasRule(diagnostics, "SIP-COMPAT-001") {
		t.Fatalf("CompareManifests() diagnostics = %#v, want method-behavior major transition error", diagnostics)
	}
}

func TestCompareManifests_logicalFieldChangeRequiresMajorTransition(t *testing.T) {
	baseline := comparisonManifest(false)
	baseline.GetServices()[0].Fields = []*manifestv1alpha1.ServiceFieldManifest{{
		Field: &optionsv1alpha1.ServiceField{
			Name:         "state",
			ValueType:    &optionsv1alpha1.ProtobufReference{Fqn: ".example.Payload"},
			GetterMethod: &optionsv1alpha1.ProtobufReference{Fqn: ".example.Service.Get"},
		},
	}}
	current := proto.Clone(baseline).(*manifestv1alpha1.Manifest)
	current.GetServices()[0].GetFields()[0].GetField().GetterMethod.Fqn = ".example.Service.Read"

	if diagnostics := CompareManifests(baseline, current); !hasRule(diagnostics, "SIP-COMPAT-001") {
		t.Fatalf("CompareManifests() diagnostics = %#v, want field-contract major transition error", diagnostics)
	}
}

func TestCompareManifests_memberIDSwapRequiresMajorTransition(t *testing.T) {
	baseline := comparisonManifest(false)
	baseline.GetServices()[0].Members = append(baseline.GetServices()[0].Members, &manifestv1alpha1.ServiceMemberManifest{
		Name:     "Set",
		MemberId: 2,
		Kind:     "method",
	})
	current := proto.Clone(baseline).(*manifestv1alpha1.Manifest)
	current.GetServices()[0].GetMembers()[0].MemberId = 2
	current.GetServices()[0].GetMembers()[1].MemberId = 1

	if diagnostics := CompareManifests(baseline, current); !hasRule(diagnostics, "SIP-COMPAT-001") {
		t.Fatalf("CompareManifests() diagnostics = %#v, want member-ID swap major transition error", diagnostics)
	}
}

func TestCompareManifests_reachableEnumChangeRequiresMajorTransition(t *testing.T) {
	baseline := comparisonManifest(false)
	member := baseline.GetPayloadTypes()[0].GetLayoutElements()[0].GetMember()
	member.Type = &optionsv1alpha1.ProtobufReference{Fqn: ".example.Mode"}
	member.Options.ScalarRepresentation = optionsv1alpha1.ScalarRepresentation_SCALAR_REPRESENTATION_ENUM_8
	baseline.EnumTypes = []*manifestv1alpha1.EnumManifest{{
		Type:    &optionsv1alpha1.ProtobufReference{Fqn: ".example.Mode"},
		Options: &optionsv1alpha1.EnumOptionsSpec{WireWidthBits: 8},
		Values: []*manifestv1alpha1.EnumValueManifest{
			{Name: "MODE_UNSPECIFIED", Number: 0, Options: &optionsv1alpha1.EnumValueOptionsSpec{NonSendablePlaceholder: true}},
			{Name: "MODE_ENABLED", Number: 1},
		},
	}}
	current := proto.Clone(baseline).(*manifestv1alpha1.Manifest)
	current.GetEnumTypes()[0].GetOptions().Closed = true

	diagnostics := CompareManifests(baseline, current)
	if !hasRule(diagnostics, "SIP-COMPAT-002") || !hasRule(diagnostics, "SIP-COMPAT-001") {
		t.Fatalf("CompareManifests() diagnostics = %#v, want enum static-evolution and major-transition errors", diagnostics)
	}
}

func TestCompareManifests_ignoresEndpointAndOverrideOrdering(t *testing.T) {
	baseline := comparisonManifest(false)
	baseline.GetProfile().ProviderInstances = []*manifestv1alpha1.ProviderInstanceManifest{{
		Instance: &optionsv1alpha1.ProviderInstance{
			ServiceId:           1,
			InstanceId:          1,
			ProviderName:        "provider",
			DefaultEndpointName: "first",
			Endpoints: []*optionsv1alpha1.Endpoint{
				{Name: "first", Address: "192.0.2.1", Transport: optionsv1alpha1.TransportProtocol_TRANSPORT_PROTOCOL_UDP, Port: 3000},
				{Name: "second", Address: "192.0.2.2", Transport: optionsv1alpha1.TransportProtocol_TRANSPORT_PROTOCOL_TCP, Port: 3001},
			},
		},
		ServiceVersionBindings: []*manifestv1alpha1.ProviderServiceVersionBindingManifest{{
			Binding: &optionsv1alpha1.ServiceVersionBinding{
				Service:               &optionsv1alpha1.ProtobufReference{Fqn: ".example.Service"},
				InterfaceMajorVersion: 1,
				MemberEndpointOverrides: []*optionsv1alpha1.MemberEndpointOverride{
					{MemberId: 1, EndpointName: "first"},
					{MemberId: 2, EndpointName: "second"},
				},
			},
			ServiceLineage: &optionsv1alpha1.ServiceLineage{Id: "example"},
		}},
	}}
	current := proto.Clone(baseline).(*manifestv1alpha1.Manifest)
	provider := current.GetProfile().GetProviderInstances()[0]
	provider.GetInstance().Endpoints[0], provider.GetInstance().Endpoints[1] = provider.GetInstance().Endpoints[1], provider.GetInstance().Endpoints[0]
	overrides := provider.GetServiceVersionBindings()[0].GetBinding().MemberEndpointOverrides
	overrides[0], overrides[1] = overrides[1], overrides[0]

	if diagnostics := CompareManifests(baseline, current); len(diagnostics) != 0 {
		t.Fatalf("CompareManifests() diagnostics = %#v, want none for ordering-only change", diagnostics)
	}
}

func TestCompareManifests_staticSDTPChangesRequireProfileMajorTransition(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*manifestv1alpha1.Manifest)
	}{
		{
			name: "consumer selector",
			mutate: func(manifest *manifestv1alpha1.Manifest) {
				manifest.GetProfile().GetConsumerBindings()[0].GetProviderSelector().GetSelector().Selector = &optionsv1alpha1.ProviderSelector_NamedProviderName{NamedProviderName: "other-provider"}
			},
		},
		{
			name: "consumer local endpoint",
			mutate: func(manifest *manifestv1alpha1.Manifest) {
				manifest.GetProfile().GetConsumerBindings()[0].GetLocalEndpoints()[0].Port = &optionsv1alpha1.ConsumerEndpoint_FixedPort{FixedPort: 30511}
			},
		},
		{
			name: "consumer find timing",
			mutate: func(manifest *manifestv1alpha1.Manifest) {
				manifest.GetProfile().GetConsumerBindings()[0].GetFindTiming().LiveTtlSeconds = 11
			},
		},
		{
			name: "consumer subscription",
			mutate: func(manifest *manifestv1alpha1.Manifest) {
				manifest.GetProfile().GetConsumerBindings()[0].GetSubscriptions()[0].GetSubscription().TtlSeconds = 6
			},
		},
		{
			name: "provider offer timing",
			mutate: func(manifest *manifestv1alpha1.Manifest) {
				manifest.GetProfile().GetProviderInstances()[0].GetServiceVersionBindings()[0].GetBinding().GetServiceDiscoveryOffer().Timing.LiveTtlSeconds = 11
			},
		},
		{
			name: "provider delivery policy",
			mutate: func(manifest *manifestv1alpha1.Manifest) {
				policy := manifest.GetProfile().GetProviderInstances()[0].GetServiceVersionBindings()[0].GetBinding().GetServiceDiscoveryOffer().EventGroupDeliveryPolicies[0]
				policy.Mode = optionsv1alpha1.EventGroupDeliveryMode_EVENT_GROUP_DELIVERY_MODE_MULTICAST
				policy.MulticastEndpoint = &optionsv1alpha1.MulticastEndpoint{Address: "239.1.1.1", Port: 30512}
			},
		},
		{
			name: "effective delivery policy",
			mutate: func(manifest *manifestv1alpha1.Manifest) {
				manifest.GetProfile().GetProviderInstances()[0].GetServiceVersionBindings()[0].GetEventGroupDeliveryPolicies()[0].Mode = optionsv1alpha1.EventGroupDeliveryMode_EVENT_GROUP_DELIVERY_MODE_MULTICAST
			},
		},
		{
			name: "provider TP declaration",
			mutate: func(manifest *manifestv1alpha1.Manifest) {
				manifest.GetProfile().GetProviderInstances()[0].GetServiceVersionBindings()[0].GetBinding().MemberTpConfigurations[0].TpWhenNeeded = false
			},
		},
		{
			name: "derived transport classification",
			mutate: func(manifest *manifestv1alpha1.Manifest) {
				manifest.GetProfile().GetProviderInstances()[0].GetServiceVersionBindings()[0].GetMemberTpAnalysis()[0].GetDirections()[0].EffectiveTransport = manifestv1alpha1.EffectiveTransportClassification_EFFECTIVE_TRANSPORT_CLASSIFICATION_TCP
			},
		},
		{
			name: "derived TP use result",
			mutate: func(manifest *manifestv1alpha1.Manifest) {
				manifest.GetProfile().GetProviderInstances()[0].GetServiceVersionBindings()[0].GetMemberTpAnalysis()[0].GetDirections()[0].TpUseResult = manifestv1alpha1.TpUseResult_TP_USE_RESULT_NEVER_NEEDED
			},
		},
		{
			name: "consumer addition",
			mutate: func(manifest *manifestv1alpha1.Manifest) {
				additional := proto.Clone(manifest.GetProfile().GetConsumerBindings()[0]).(*manifestv1alpha1.ConsumerServiceBindingManifest)
				additional.GetBinding().ConsumerName = "comparison.consumer.second"
				manifest.GetProfile().ConsumerBindings = append(manifest.GetProfile().ConsumerBindings, additional)
			},
		},
		{
			name: "subscription addition",
			mutate: func(manifest *manifestv1alpha1.Manifest) {
				manifest.GetProfile().GetConsumerBindings()[0].Subscriptions = append(manifest.GetProfile().GetConsumerBindings()[0].GetSubscriptions(), &manifestv1alpha1.ConsumerSubscriptionManifest{
					Subscription: &optionsv1alpha1.Subscription{EventGroupName: "secondary", TtlSeconds: 5, LocalEndpointNames: []string{"consumer-udp"}},
					EventGroupId: 2,
				})
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			baseline := sdTPComparisonManifest()
			current := proto.Clone(baseline).(*manifestv1alpha1.Manifest)
			test.mutate(current)

			if diagnostics := CompareManifests(baseline, current); !hasRule(diagnostics, "SIP-COMPAT-001") {
				t.Fatalf("CompareManifests() diagnostics = %#v, want profile-major transition error", diagnostics)
			}
			current.GetProfile().GetVersion().Major = 2
			if diagnostics := CompareManifests(baseline, current); len(diagnostics) != 0 {
				t.Fatalf("CompareManifests() diagnostics = %#v, want accepted profile-major transition", diagnostics)
			}
		})
	}
}

func TestCompareManifests_ignoresConsumerContractOrdering(t *testing.T) {
	baseline := sdTPComparisonManifest()
	current := proto.Clone(baseline).(*manifestv1alpha1.Manifest)
	consumer := current.GetProfile().GetConsumerBindings()[0]
	consumer.LocalEndpoints = append(consumer.LocalEndpoints, &optionsv1alpha1.ConsumerEndpoint{
		Name:      "consumer-tcp",
		Address:   "192.0.2.21",
		Transport: optionsv1alpha1.TransportProtocol_TRANSPORT_PROTOCOL_TCP,
		Port:      &optionsv1alpha1.ConsumerEndpoint_FixedPort{FixedPort: 30511},
	})
	baseline.GetProfile().GetConsumerBindings()[0].LocalEndpoints = append([]*optionsv1alpha1.ConsumerEndpoint(nil), consumer.LocalEndpoints...)
	current.GetProfile().GetConsumerBindings()[0].LocalEndpoints[0], current.GetProfile().GetConsumerBindings()[0].LocalEndpoints[1] = current.GetProfile().GetConsumerBindings()[0].LocalEndpoints[1], current.GetProfile().GetConsumerBindings()[0].LocalEndpoints[0]

	if diagnostics := CompareManifests(baseline, current); len(diagnostics) != 0 {
		t.Fatalf("CompareManifests() diagnostics = %#v, want none for consumer endpoint ordering-only change", diagnostics)
	}
}

func TestCompareVersion_preReleasePrecedence(t *testing.T) {
	versions := []*optionsv1alpha1.SemanticVersion{
		{Major: 1, Prerelease: "alpha.1"},
		{Major: 1, Prerelease: "alpha.beta"},
		{Major: 1, Prerelease: "beta"},
		{Major: 1, Prerelease: "beta.2"},
		{Major: 1, Prerelease: "beta.11"},
		{Major: 1, Prerelease: "rc.1"},
		{Major: 1},
	}
	for index := 1; index < len(versions); index++ {
		if got := compareVersion(versions[index], versions[index-1]); got <= 0 {
			t.Fatalf("compareVersion(%s, %s) = %d, want positive", versionText(versions[index]), versionText(versions[index-1]), got)
		}
	}
	left := &optionsv1alpha1.SemanticVersion{Major: 1, BuildMetadata: "build.1"}
	right := &optionsv1alpha1.SemanticVersion{Major: 1, BuildMetadata: "build.2"}
	if got := compareVersion(left, right); got != 0 {
		t.Fatalf("compareVersion() = %d for build-only change, want zero", got)
	}
}

func comparisonManifest(extensible bool) *manifestv1alpha1.Manifest {
	lengthWidth := optionsv1alpha1.LengthFieldWidth_LENGTH_FIELD_WIDTH_NONE
	if extensible {
		lengthWidth = optionsv1alpha1.LengthFieldWidth_LENGTH_FIELD_WIDTH_32
	}
	payloadOptions := &optionsv1alpha1.MessageOptionsSpec{StructLayout: &optionsv1alpha1.StructLayout{
		LengthFieldWidth: lengthWidth,
		Extensible:       extensible,
	}}
	return &manifestv1alpha1.Manifest{
		Profile: &manifestv1alpha1.ProfileManifest{
			ProfileName: "comparison",
			Version:     &optionsv1alpha1.SemanticVersion{Major: 1},
		},
		Services: []*manifestv1alpha1.ServiceManifest{{
			Service:               &optionsv1alpha1.ProtobufReference{Fqn: ".example.Service"},
			ServiceLineage:        &optionsv1alpha1.ServiceLineage{Id: "example"},
			InterfaceMajorVersion: 1,
			Members: []*manifestv1alpha1.ServiceMemberManifest{{
				Name:               "Get",
				MemberId:           1,
				Kind:               "method",
				RequestPayloadType: &optionsv1alpha1.ProtobufReference{Fqn: ".example.Payload"},
			}},
		}},
		PayloadTypes: []*manifestv1alpha1.PayloadTypeManifest{{
			Type:        &optionsv1alpha1.ProtobufReference{Fqn: ".example.Payload"},
			Options:     payloadOptions,
			EncodedSize: &manifestv1alpha1.EncodedSize{ExactBytes: 1, MinimumBytes: 1, MaximumBytes: 1},
			EffectiveLayout: &manifestv1alpha1.EffectivePayloadLayout{
				ByteOrder:        optionsv1alpha1.ByteOrder_BYTE_ORDER_BIG_ENDIAN,
				LengthFieldWidth: lengthWidth,
				Extensible:       extensible,
				MaximumWireOrder: 1,
			},
			LayoutElements: []*manifestv1alpha1.PayloadLayoutElement{payloadMemberElement("first", 1, 1)},
		}},
	}
}

func sdTPComparisonManifest() *manifestv1alpha1.Manifest {
	manifest := comparisonManifest(false)
	manifest.GetProfile().ProviderInstances = []*manifestv1alpha1.ProviderInstanceManifest{{
		Instance: &optionsv1alpha1.ProviderInstance{
			ServiceId:           1,
			InstanceId:          1,
			ProviderName:        "comparison.provider",
			DefaultEndpointName: "provider-udp",
			Endpoints: []*optionsv1alpha1.Endpoint{{
				Name:      "provider-udp",
				Address:   "192.0.2.10",
				Transport: optionsv1alpha1.TransportProtocol_TRANSPORT_PROTOCOL_UDP,
				Port:      30500,
			}},
		},
		ServiceVersionBindings: []*manifestv1alpha1.ProviderServiceVersionBindingManifest{{
			Binding: &optionsv1alpha1.ServiceVersionBinding{
				Service:               &optionsv1alpha1.ProtobufReference{Fqn: ".example.Service"},
				InterfaceMajorVersion: 1,
				ServiceDiscoveryOffer: &optionsv1alpha1.ServiceDiscoveryOffer{
					EndpointNames: []string{"provider-udp"},
					Timing:        &optionsv1alpha1.ServiceDiscoveryTiming{LiveTtlSeconds: 10},
					EventGroupDeliveryPolicies: []*optionsv1alpha1.EventGroupDeliveryPolicy{{
						EventGroupName: "updates",
						Mode:           optionsv1alpha1.EventGroupDeliveryMode_EVENT_GROUP_DELIVERY_MODE_UNICAST,
					}},
				},
				MemberTpConfigurations: []*optionsv1alpha1.MemberTpConfiguration{{MemberId: 1, TpWhenNeeded: true}},
			},
			ServiceLineage: &optionsv1alpha1.ServiceLineage{Id: "example"},
			ServiceDiscoveryOffer: &optionsv1alpha1.ServiceDiscoveryOffer{
				EndpointNames: []string{"provider-udp"},
				Timing:        &optionsv1alpha1.ServiceDiscoveryTiming{LiveTtlSeconds: 10},
			},
			EventGroupDeliveryPolicies: []*optionsv1alpha1.EventGroupDeliveryPolicy{{
				EventGroupName: "updates",
				Mode:           optionsv1alpha1.EventGroupDeliveryMode_EVENT_GROUP_DELIVERY_MODE_UNICAST,
			}},
			MemberTpConfigurations: []*optionsv1alpha1.MemberTpConfiguration{{MemberId: 1, TpWhenNeeded: true}},
			MemberTpAnalysis: []*manifestv1alpha1.ProviderMemberTpAnalysisManifest{{
				MemberId:     1,
				TpWhenNeeded: true,
				Directions: []*manifestv1alpha1.MemberDirectionTpAnalysisManifest{{
					Direction:          manifestv1alpha1.MemberDirection_MEMBER_DIRECTION_REQUEST,
					EffectiveTransport: manifestv1alpha1.EffectiveTransportClassification_EFFECTIVE_TRANSPORT_CLASSIFICATION_UDP,
					TpWhenNeeded:       true,
					TpUseResult:        manifestv1alpha1.TpUseResult_TP_USE_RESULT_MAY_BE_NEEDED,
				}},
			}},
		}},
	}}
	manifest.GetProfile().ConsumerBindings = []*manifestv1alpha1.ConsumerServiceBindingManifest{{
		Binding: &optionsv1alpha1.ConsumerServiceBinding{
			ConsumerName:          "comparison.consumer",
			Service:               &optionsv1alpha1.ProtobufReference{Fqn: ".example.Service"},
			ServiceLineage:        &optionsv1alpha1.ServiceLineage{Id: "example"},
			InterfaceMajorVersion: 1,
			DefaultEndpointName:   "consumer-udp",
		},
		ProviderSelector: &manifestv1alpha1.ResolvedProviderSelectorManifest{
			Selector: &optionsv1alpha1.ProviderSelector{Selector: &optionsv1alpha1.ProviderSelector_NamedProviderName{NamedProviderName: "comparison.provider"}},
			Candidates: []*manifestv1alpha1.ProviderCandidateManifest{{
				ProviderName:          "comparison.provider",
				ServiceId:             1,
				InstanceId:            1,
				Service:               &optionsv1alpha1.ProtobufReference{Fqn: ".example.Service"},
				InterfaceMajorVersion: 1,
				ServiceLineage:        &optionsv1alpha1.ServiceLineage{Id: "example"},
			}},
		},
		LocalEndpoints: []*optionsv1alpha1.ConsumerEndpoint{{
			Name:      "consumer-udp",
			Address:   "192.0.2.20",
			Transport: optionsv1alpha1.TransportProtocol_TRANSPORT_PROTOCOL_UDP,
			Port:      &optionsv1alpha1.ConsumerEndpoint_FixedPort{FixedPort: 30510},
		}},
		FindTiming: &optionsv1alpha1.ServiceDiscoveryTiming{LiveTtlSeconds: 10},
		Subscriptions: []*manifestv1alpha1.ConsumerSubscriptionManifest{{
			Subscription:   &optionsv1alpha1.Subscription{EventGroupName: "updates", TtlSeconds: 5, LocalEndpointNames: []string{"consumer-udp"}},
			EventGroupId:   1,
			DeliveryPolicy: &optionsv1alpha1.EventGroupDeliveryPolicy{EventGroupName: "updates", Mode: optionsv1alpha1.EventGroupDeliveryMode_EVENT_GROUP_DELIVERY_MODE_UNICAST},
		}},
		InheritedMemberTpAnalysis: []*manifestv1alpha1.ProviderMemberTpAnalysisManifest{{
			MemberId:     1,
			TpWhenNeeded: true,
			Directions: []*manifestv1alpha1.MemberDirectionTpAnalysisManifest{{
				Direction:          manifestv1alpha1.MemberDirection_MEMBER_DIRECTION_REQUEST,
				EffectiveTransport: manifestv1alpha1.EffectiveTransportClassification_EFFECTIVE_TRANSPORT_CLASSIFICATION_UDP,
				TpWhenNeeded:       true,
				TpUseResult:        manifestv1alpha1.TpUseResult_TP_USE_RESULT_MAY_BE_NEEDED,
			}},
		}},
	}}
	return manifest
}

func payloadMemberElement(name string, fieldNumber int32, wireOrder uint32) *manifestv1alpha1.PayloadLayoutElement {
	member := &manifestv1alpha1.PayloadMemberManifest{
		Name:        name,
		FieldNumber: uint32(fieldNumber),
		Options: &optionsv1alpha1.FieldOptionsSpec{
			WireOrder:            wireOrder,
			ScalarRepresentation: optionsv1alpha1.ScalarRepresentation_SCALAR_REPRESENTATION_UNSIGNED_INTEGER_8,
		},
		EncodedSize: &manifestv1alpha1.EncodedSize{ExactBytes: 1, MinimumBytes: 1, MaximumBytes: 1},
	}
	return &manifestv1alpha1.PayloadLayoutElement{
		Element: &manifestv1alpha1.PayloadLayoutElement_Member{Member: member},
	}
}

func hasRule(diagnostics []*manifestv1alpha1.Diagnostic, rule string) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.GetRuleCode() == rule {
			return true
		}
	}
	return false
}
