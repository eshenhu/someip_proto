package semantic

import (
	"testing"
	"time"

	manifestv1alpha1 "github.com/eshenhu/someip_proto/gen/go/io/github/someip_proto/manifest/v1alpha1"
	optionsv1alpha1 "github.com/eshenhu/someip_proto/gen/go/io/github/someip_proto/options/v1alpha1"
	testv1alpha1 "github.com/eshenhu/someip_proto/gen/go/io/github/someip_proto/test/v1alpha1"
	"github.com/eshenhu/someip_proto/internal/descriptor"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/known/durationpb"
)

func TestProviderTPDirectionAnalysis_derivesStaticUse(t *testing.T) {
	udp := &optionsv1alpha1.Endpoint{
		Name:      "udp",
		Address:   "192.0.2.1",
		Transport: optionsv1alpha1.TransportProtocol_TRANSPORT_PROTOCOL_UDP,
		Port:      30500,
	}
	tcp := &optionsv1alpha1.Endpoint{
		Name:      "tcp",
		Address:   "192.0.2.1",
		Transport: optionsv1alpha1.TransportProtocol_TRANSPORT_PROTOCOL_TCP,
		Port:      30501,
	}
	whenNeeded := &optionsv1alpha1.MemberTpConfiguration{TpWhenNeeded: true}

	for _, test := range []struct {
		name       string
		endpoint   *optionsv1alpha1.Endpoint
		config     *optionsv1alpha1.MemberTpConfiguration
		minimum    uint64
		maximum    uint64
		transport  manifestv1alpha1.EffectiveTransportClassification
		result     manifestv1alpha1.TpUseResult
		runtimeTP  bool
		wantBudget uint64
	}{
		{
			name:       "udp without declaration",
			endpoint:   udp,
			minimum:    0,
			maximum:    101,
			transport:  manifestv1alpha1.EffectiveTransportClassification_EFFECTIVE_TRANSPORT_CLASSIFICATION_UDP,
			result:     manifestv1alpha1.TpUseResult_TP_USE_RESULT_TP_NOT_CONFIGURED,
			wantBudget: 100,
		},
		{
			name:       "udp configured never needed",
			endpoint:   udp,
			config:     whenNeeded,
			minimum:    0,
			maximum:    100,
			transport:  manifestv1alpha1.EffectiveTransportClassification_EFFECTIVE_TRANSPORT_CLASSIFICATION_UDP,
			result:     manifestv1alpha1.TpUseResult_TP_USE_RESULT_NEVER_NEEDED,
			wantBudget: 100,
		},
		{
			name:       "udp configured may be needed",
			endpoint:   udp,
			config:     whenNeeded,
			minimum:    100,
			maximum:    101,
			transport:  manifestv1alpha1.EffectiveTransportClassification_EFFECTIVE_TRANSPORT_CLASSIFICATION_UDP,
			result:     manifestv1alpha1.TpUseResult_TP_USE_RESULT_MAY_BE_NEEDED,
			runtimeTP:  true,
			wantBudget: 100,
		},
		{
			name:       "udp configured required",
			endpoint:   udp,
			config:     whenNeeded,
			minimum:    101,
			maximum:    101,
			transport:  manifestv1alpha1.EffectiveTransportClassification_EFFECTIVE_TRANSPORT_CLASSIFICATION_UDP,
			result:     manifestv1alpha1.TpUseResult_TP_USE_RESULT_REQUIRED,
			runtimeTP:  true,
			wantBudget: 100,
		},
		{
			name:      "tcp not applicable",
			endpoint:  tcp,
			config:    whenNeeded,
			minimum:   101,
			maximum:   101,
			transport: manifestv1alpha1.EffectiveTransportClassification_EFFECTIVE_TRANSPORT_CLASSIFICATION_TCP,
			result:    manifestv1alpha1.TpUseResult_TP_USE_RESULT_NOT_APPLICABLE,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			analysis := providerTPDirectionAnalysis(memberDirectionalBound{
				direction: manifestv1alpha1.MemberDirection_MEMBER_DIRECTION_REQUEST,
				size:      encodedSize{minimum: test.minimum, maximum: test.maximum},
			}, test.endpoint, test.config, 100)
			if analysis.GetEffectiveTransport() != test.transport || analysis.GetTpUseResult() != test.result || analysis.GetRuntimeTpSessionHandlingRequired() != test.runtimeTP || analysis.GetUnsegmentedUdpPayloadBudgetBytes() != test.wantBudget {
				t.Fatalf("analysis = %#v, want transport=%s result=%s runtime=%t budget=%d", analysis, test.transport, test.result, test.runtimeTP, test.wantBudget)
			}
		})
	}
}

func TestNormalizeProviderBindingTP_validatesDeclarationCoverage(t *testing.T) {
	provider := &optionsv1alpha1.ProviderInstance{
		DefaultEndpointName: "udp",
		Endpoints: []*optionsv1alpha1.Endpoint{
			{Name: "udp", Address: "192.0.2.1", Transport: optionsv1alpha1.TransportProtocol_TRANSPORT_PROTOCOL_UDP, Port: 30500},
			{Name: "tcp", Address: "192.0.2.1", Transport: optionsv1alpha1.TransportProtocol_TRANSPORT_PROTOCOL_TCP, Port: 30501},
		},
	}
	binding := &optionsv1alpha1.ServiceVersionBinding{
		MemberEndpointOverrides: []*optionsv1alpha1.MemberEndpointOverride{{MemberId: 2, EndpointName: "tcp"}},
		MemberTpConfigurations: []*optionsv1alpha1.MemberTpConfiguration{
			{MemberId: 1, TpWhenNeeded: false},
			{MemberId: 1, TpWhenNeeded: true},
			{MemberId: 2, TpWhenNeeded: false},
			{MemberId: 99, TpWhenNeeded: true},
		},
	}
	service := &serviceInfo{
		manifest: &manifestv1alpha1.ServiceManifest{Members: []*manifestv1alpha1.ServiceMemberManifest{
			{MemberId: 1},
			{MemberId: 2},
			{MemberId: 3},
		}},
		memberPayloadBounds: map[uint32][]memberDirectionalBound{
			1: {{direction: manifestv1alpha1.MemberDirection_MEMBER_DIRECTION_REQUEST, size: fixedSize(101)}},
			2: {{direction: manifestv1alpha1.MemberDirection_MEMBER_DIRECTION_REQUEST, size: fixedSize(10)}},
			3: {{direction: manifestv1alpha1.MemberDirection_MEMBER_DIRECTION_REQUEST, size: fixedSize(101)}},
		},
	}
	manifest := &manifestv1alpha1.ProviderServiceVersionBindingManifest{}
	normalizer := &normalizer{}
	normalizer.normalizeProviderBindingTP(&providerBindingInfo{
		provider:        provider,
		binding:         binding,
		bindingManifest: manifest,
		service:         service,
		endpoints: map[string]*optionsv1alpha1.Endpoint{
			"udp": provider.GetEndpoints()[0],
			"tcp": provider.GetEndpoints()[1],
		},
	}, 100)

	if !hasDiagnostic(normalizer.diagnostics.diagnostics, "SIP-TP-001", manifestv1alpha1.DiagnosticSeverity_DIAGNOSTIC_SEVERITY_ERROR) {
		t.Fatalf("diagnostics = %s, want TP declaration-integrity errors", diagnosticSummary(normalizer.diagnostics.diagnostics))
	}
	if !hasDiagnostic(normalizer.diagnostics.diagnostics, "SIP-TP-003", manifestv1alpha1.DiagnosticSeverity_DIAGNOSTIC_SEVERITY_ERROR) {
		t.Fatalf("diagnostics = %s, want missing TP declaration error", diagnosticSummary(normalizer.diagnostics.diagnostics))
	}
	if !hasDiagnostic(normalizer.diagnostics.diagnostics, "SIP-TP-002", manifestv1alpha1.DiagnosticSeverity_DIAGNOSTIC_SEVERITY_ERROR) {
		t.Fatalf("diagnostics = %s, want oversized TP-disabled UDP flow error", diagnosticSummary(normalizer.diagnostics.diagnostics))
	}
	if got := len(manifest.GetMemberTpAnalysis()); got != 3 {
		t.Fatalf("member TP analyses = %d, want 3", got)
	}
}

func TestNormalizeProviderBindingSD_materializesValidStaticOffer(t *testing.T) {
	service := &serviceInfo{
		manifest: &manifestv1alpha1.ServiceManifest{
			Members:     []*manifestv1alpha1.ServiceMemberManifest{{MemberId: 1}},
			EventGroups: []*optionsv1alpha1.EventGroup{{Name: "state", Id: 1}},
		},
	}
	provider := &optionsv1alpha1.ProviderInstance{DefaultEndpointName: "udp"}
	endpoint := &optionsv1alpha1.Endpoint{
		Name:      "udp",
		Address:   "192.0.2.1",
		Transport: optionsv1alpha1.TransportProtocol_TRANSPORT_PROTOCOL_UDP,
		Port:      30500,
	}
	provider.Endpoints = []*optionsv1alpha1.Endpoint{endpoint}
	binding := &optionsv1alpha1.ServiceVersionBinding{ServiceDiscoveryOffer: &optionsv1alpha1.ServiceDiscoveryOffer{
		EndpointNames: []string{"udp"},
		Timing:        completeServiceDiscoveryTiming(),
		EventGroupDeliveryPolicies: []*optionsv1alpha1.EventGroupDeliveryPolicy{{
			EventGroupName: "state",
			Mode:           optionsv1alpha1.EventGroupDeliveryMode_EVENT_GROUP_DELIVERY_MODE_UNICAST,
		}},
	}}
	manifest := &manifestv1alpha1.ProviderServiceVersionBindingManifest{}
	normalizer := &normalizer{}
	normalizer.normalizeProviderBindingSD(&providerBindingInfo{
		provider:        provider,
		binding:         binding,
		bindingManifest: manifest,
		service:         service,
		endpoints:       map[string]*optionsv1alpha1.Endpoint{"udp": endpoint},
	})
	if len(normalizer.diagnostics.diagnostics) != 0 {
		t.Fatalf("diagnostics = %s, want none", diagnosticSummary(normalizer.diagnostics.diagnostics))
	}
	if manifest.GetServiceDiscoveryOffer() == nil || len(manifest.GetEventGroupDeliveryPolicies()) != 1 {
		t.Fatalf("provider SD manifest = %#v, want materialized offer and delivery policy", manifest)
	}
}

func TestNormalize_materializesStaticSDConsumerTopology(t *testing.T) {
	set := mutableFixtureDescriptorSet(t)
	profile := deploymentProfileOptions(t, fixtureFile(t, set, "io/github/someip_proto/test/v1alpha1/phase_2_1_schema_fixture.proto"))
	provider := profile.GetProviderInstances()[0]
	provider.GetServiceVersionBindings()[0].ServiceDiscoveryOffer = &optionsv1alpha1.ServiceDiscoveryOffer{
		EndpointNames: []string{"primary"},
		Timing:        completeServiceDiscoveryTiming(),
	}
	profile.ConsumerBindings = []*optionsv1alpha1.ConsumerServiceBinding{{
		ConsumerName:          "fixture.consumer",
		Service:               &optionsv1alpha1.ProtobufReference{Fqn: ".io.github.someip_proto.test.v1alpha1.PhaseTwoOneV1Service"},
		ServiceLineage:        &optionsv1alpha1.ServiceLineage{Id: "fixture.lighting"},
		InterfaceMajorVersion: 1,
		ProviderSelector: &optionsv1alpha1.ProviderSelector{
			Selector: &optionsv1alpha1.ProviderSelector_NamedProviderName{NamedProviderName: "fixture.provider"},
		},
		LocalEndpoints: []*optionsv1alpha1.ConsumerEndpoint{{
			Name:      "local",
			Address:   "192.0.2.20",
			Transport: optionsv1alpha1.TransportProtocol_TRANSPORT_PROTOCOL_UDP,
			Port:      &optionsv1alpha1.ConsumerEndpoint_FixedPort{FixedPort: 30510},
		}},
		DefaultEndpointName: "local",
		FindTiming:          &optionsv1alpha1.ServiceDiscoveryTiming{LiveTtlSeconds: 10},
	}}

	result := normalizeMutableFixture(t, set)
	if result.HasErrors() {
		t.Fatalf("Normalize() diagnostics = %s, want static SD consumer topology", diagnosticSummary(result.Diagnostics))
	}
	consumers := result.Manifest.GetProfile().GetConsumerBindings()
	if len(consumers) != 1 || len(consumers[0].GetProviderSelector().GetCandidates()) != 1 || len(consumers[0].GetInheritedMemberTpAnalysis()) != 1 {
		t.Fatalf("consumer bindings = %#v, want resolved provider and inherited TP contract", consumers)
	}
	if got := consumers[0].GetInheritedMemberTpAnalysis()[0].GetDirections()[0].GetTpUseResult(); got != manifestv1alpha1.TpUseResult_TP_USE_RESULT_TP_NOT_CONFIGURED {
		t.Fatalf("inherited TP result = %s, want TP_NOT_CONFIGURED", got)
	}
	request := tpDirection(consumers[0].GetInheritedMemberTpAnalysis()[0], manifestv1alpha1.MemberDirection_MEMBER_DIRECTION_REQUEST)
	if request == nil || request.GetEndpoint().GetAddress() != "192.0.2.20" {
		t.Fatalf("inherited request analysis = %#v, want consumer endpoint", request)
	}
	response := tpDirection(consumers[0].GetInheritedMemberTpAnalysis()[0], manifestv1alpha1.MemberDirection_MEMBER_DIRECTION_RESPONSE)
	if response == nil || response.GetEndpoint().GetAddress() != "192.0.2.10" {
		t.Fatalf("inherited response analysis = %#v, want provider endpoint", response)
	}
}

func TestValidateConsumerRequestEndpointCoverage_rejectsMissingEndpoint(t *testing.T) {
	service := &serviceInfo{
		manifest: &manifestv1alpha1.ServiceManifest{Members: []*manifestv1alpha1.ServiceMemberManifest{{MemberId: 1}}},
		memberPayloadBounds: map[uint32][]memberDirectionalBound{
			1: {{direction: manifestv1alpha1.MemberDirection_MEMBER_DIRECTION_REQUEST, size: fixedSize(1)}},
		},
	}
	normalizer := &normalizer{}
	normalizer.validateConsumerRequestEndpointCoverage(service, &optionsv1alpha1.ConsumerServiceBinding{}, nil, descriptor.Location{})
	if !hasDiagnostic(normalizer.diagnostics.diagnostics, "SIP-SD-001", manifestv1alpha1.DiagnosticSeverity_DIAGNOSTIC_SEVERITY_ERROR) {
		t.Fatalf("diagnostics = %s, want missing request endpoint error", diagnosticSummary(normalizer.diagnostics.diagnostics))
	}
}

func TestValidateSubscriptionEndpoints_requiresLocalEndpointForInitialData(t *testing.T) {
	normalizer := &normalizer{}
	normalizer.validateSubscriptionEndpoints(&optionsv1alpha1.Subscription{
		EventGroupName:     "state",
		RequestInitialData: true,
	}, nil, true, descriptor.Location{})
	if !hasDiagnostic(normalizer.diagnostics.diagnostics, "SIP-SD-003", manifestv1alpha1.DiagnosticSeverity_DIAGNOSTIC_SEVERITY_ERROR) {
		t.Fatalf("diagnostics = %s, want initial-data local endpoint error", diagnosticSummary(normalizer.diagnostics.diagnostics))
	}
}

func TestNormalize_staticSDTPFixture_materializesCompleteContract(t *testing.T) {
	set := loadSDTPFixtureSet(t)
	result := Normalize(set, "sd-tp-valid", DefaultConfig())
	if result.HasErrors() {
		t.Fatalf("Normalize() diagnostics = %s, want valid complete static SD/TP contract", diagnosticSummary(result.Diagnostics))
	}
	profile := result.Manifest.GetProfile()
	if len(profile.GetProviderInstances()) != 2 || len(profile.GetConsumerBindings()) != 2 {
		t.Fatalf("profile = %#v, want two providers and two consumers", profile)
	}

	provider := providerBindingByName(t, profile, "sd-tp.udp-one")
	if provider.GetServiceDiscoveryOffer() == nil || len(provider.GetEventGroupDeliveryPolicies()) != 2 {
		t.Fatalf("provider SD contract = %#v, want offer and two delivery policies", provider)
	}
	assertTPDirectionResult(t, provider, 1, manifestv1alpha1.MemberDirection_MEMBER_DIRECTION_REQUEST, manifestv1alpha1.TpUseResult_TP_USE_RESULT_NEVER_NEEDED, false)
	assertTPDirectionResult(t, provider, 1, manifestv1alpha1.MemberDirection_MEMBER_DIRECTION_RESPONSE, manifestv1alpha1.TpUseResult_TP_USE_RESULT_NEVER_NEEDED, false)
	assertTPDirectionResult(t, provider, 2, manifestv1alpha1.MemberDirection_MEMBER_DIRECTION_REQUEST, manifestv1alpha1.TpUseResult_TP_USE_RESULT_TP_NOT_CONFIGURED, false)
	assertTPDirectionResult(t, provider, 2, manifestv1alpha1.MemberDirection_MEMBER_DIRECTION_RESPONSE, manifestv1alpha1.TpUseResult_TP_USE_RESULT_TP_NOT_CONFIGURED, false)
	assertTPDirectionResult(t, provider, 2, manifestv1alpha1.MemberDirection_MEMBER_DIRECTION_EXCEPTION, manifestv1alpha1.TpUseResult_TP_USE_RESULT_TP_NOT_CONFIGURED, false)
	assertTPDirectionResult(t, provider, 0x8001, manifestv1alpha1.MemberDirection_MEMBER_DIRECTION_NOTIFICATION, manifestv1alpha1.TpUseResult_TP_USE_RESULT_MAY_BE_NEEDED, true)
	assertTPDirectionResult(t, provider, 0x8002, manifestv1alpha1.MemberDirection_MEMBER_DIRECTION_NOTIFICATION, manifestv1alpha1.TpUseResult_TP_USE_RESULT_REQUIRED, true)

	exact := consumerBindingByName(t, profile, "sd-tp.exact-consumer")
	request := tpDirection(memberTPAnalysisByID(t, exact.GetInheritedMemberTpAnalysis(), 1), manifestv1alpha1.MemberDirection_MEMBER_DIRECTION_REQUEST)
	if request.GetEndpoint().GetAddress() != "192.0.2.200" || request.GetEndpoint().GetFixedPort() != 30510 {
		t.Fatalf("exact consumer request endpoint = %#v, want fixed local endpoint", request.GetEndpoint())
	}
	if len(exact.GetSubscriptions()) != 2 || exact.GetSubscriptions()[0].GetDeliveryPolicy() == nil || exact.GetSubscriptions()[1].GetDeliveryPolicy() == nil {
		t.Fatalf("exact consumer subscriptions = %#v, want resolved delivery policies", exact.GetSubscriptions())
	}

	wildcard := consumerBindingByName(t, profile, "sd-tp.wildcard-consumer")
	if candidates := wildcard.GetProviderSelector().GetCandidates(); len(candidates) != 2 || candidates[0].GetProviderName() != "sd-tp.udp-one" || candidates[1].GetProviderName() != "sd-tp.udp-two" {
		t.Fatalf("wildcard candidates = %#v, want deterministic equivalent candidates", candidates)
	}
	wildcardRequest := tpDirection(memberTPAnalysisByID(t, wildcard.GetInheritedMemberTpAnalysis(), 1), manifestv1alpha1.MemberDirection_MEMBER_DIRECTION_REQUEST)
	if portRange := wildcardRequest.GetEndpoint().GetDynamicPortRange(); portRange == nil || portRange.GetMinimumPort() != 30520 || portRange.GetMaximumPort() != 30530 {
		t.Fatalf("wildcard request endpoint = %#v, want dynamic consumer port range", wildcardRequest.GetEndpoint())
	}
	wildcardResponse := tpDirection(memberTPAnalysisByID(t, wildcard.GetInheritedMemberTpAnalysis(), 1), manifestv1alpha1.MemberDirection_MEMBER_DIRECTION_RESPONSE)
	if wildcardResponse.GetEndpoint() != nil {
		t.Fatalf("wildcard response endpoint = %#v, want no candidate-specific provider endpoint", wildcardResponse.GetEndpoint())
	}
}

func TestNormalize_staticSDTPFixture_materializesTCPNotApplicable(t *testing.T) {
	set := mutableSDTPFixtureDescriptorSet(t)
	file := fixtureFile(t, set, "io/github/someip_proto/test/v1alpha1/sd_tp_schema_fixture.proto")
	profile := deploymentProfileOptions(t, file)
	tcpProvider := profile.GetProviderInstances()[1]
	tcpProvider.GetEndpoints()[0].Transport = optionsv1alpha1.TransportProtocol_TRANSPORT_PROTOCOL_TCP
	tcpProvider.GetServiceVersionBindings()[0].MemberTpConfigurations = nil

	result := normalizeMutableFixtureForProfile(t, set, "sd-tp-valid")
	if !hasDiagnostic(result.Diagnostics, "SIP-TP-004", manifestv1alpha1.DiagnosticSeverity_DIAGNOSTIC_SEVERITY_ERROR) {
		t.Fatalf("Normalize() diagnostics = %s, want wildcard TP-contract error", diagnosticSummary(result.Diagnostics))
	}
	provider := providerBindingByName(t, result.Manifest.GetProfile(), "sd-tp.udp-two")
	assertTPDirectionResult(t, provider, 1, manifestv1alpha1.MemberDirection_MEMBER_DIRECTION_REQUEST, manifestv1alpha1.TpUseResult_TP_USE_RESULT_NOT_APPLICABLE, false)
}

func TestNormalize_staticSDTPFixture_rejectsInvalidContracts(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*optionsv1alpha1.DeploymentProfile)
		rule   string
	}{
		{
			name: "unknown named provider",
			mutate: func(profile *optionsv1alpha1.DeploymentProfile) {
				profile.GetConsumerBindings()[0].GetProviderSelector().Selector = &optionsv1alpha1.ProviderSelector_NamedProviderName{NamedProviderName: "missing"}
			},
			rule: "SIP-SD-001",
		},
		{
			name: "wildcard one match resolves multiple providers",
			mutate: func(profile *optionsv1alpha1.DeploymentProfile) {
				profile.GetConsumerBindings()[1].GetProviderSelector().SelectionMode = optionsv1alpha1.ProviderSelectionMode_PROVIDER_SELECTION_MODE_ONE_MATCH
			},
			rule: "SIP-SD-001",
		},
		{
			name: "invalid offer TTL",
			mutate: func(profile *optionsv1alpha1.DeploymentProfile) {
				profile.GetProviderInstances()[0].GetServiceVersionBindings()[0].GetServiceDiscoveryOffer().GetTiming().LiveTtlSeconds = 0
			},
			rule: "SIP-SD-002",
		},
		{
			name: "invalid dynamic port range",
			mutate: func(profile *optionsv1alpha1.DeploymentProfile) {
				portRange := profile.GetConsumerBindings()[1].GetLocalEndpoints()[0].GetDynamicPortRange()
				portRange.MinimumPort, portRange.MaximumPort = portRange.MaximumPort, portRange.MinimumPort
			},
			rule: "SIP-SD-001",
		},
		{
			name: "missing delivery policy",
			mutate: func(profile *optionsv1alpha1.DeploymentProfile) {
				profile.GetProviderInstances()[0].GetServiceVersionBindings()[0].GetServiceDiscoveryOffer().EventGroupDeliveryPolicies = nil
			},
			rule: "SIP-SD-004",
		},
		{
			name: "initial data on non field group without endpoint",
			mutate: func(profile *optionsv1alpha1.DeploymentProfile) {
				profile.GetConsumerBindings()[0].GetSubscriptions()[1].RequestInitialData = true
			},
			rule: "SIP-SD-003",
		},
		{
			name: "TP disabled oversized notification",
			mutate: func(profile *optionsv1alpha1.DeploymentProfile) {
				profile.GetProviderInstances()[0].GetServiceVersionBindings()[0].GetMemberTpConfigurations()[3].TpWhenNeeded = false
			},
			rule: "SIP-TP-002",
		},
		{
			name: "missing UDP TP declaration",
			mutate: func(profile *optionsv1alpha1.DeploymentProfile) {
				configurations := profile.GetProviderInstances()[0].GetServiceVersionBindings()[0].GetMemberTpConfigurations()
				profile.GetProviderInstances()[0].GetServiceVersionBindings()[0].MemberTpConfigurations = configurations[:len(configurations)-1]
			},
			rule: "SIP-TP-003",
		},
		{
			name: "wildcard TP transport mismatch",
			mutate: func(profile *optionsv1alpha1.DeploymentProfile) {
				provider := profile.GetProviderInstances()[1]
				provider.GetEndpoints()[0].Transport = optionsv1alpha1.TransportProtocol_TRANSPORT_PROTOCOL_TCP
				provider.GetServiceVersionBindings()[0].MemberTpConfigurations = nil
			},
			rule: "SIP-TP-004",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			set := mutableSDTPFixtureDescriptorSet(t)
			file := fixtureFile(t, set, "io/github/someip_proto/test/v1alpha1/sd_tp_schema_fixture.proto")
			test.mutate(deploymentProfileOptions(t, file))
			result := normalizeMutableFixtureForProfile(t, set, "sd-tp-valid")
			if !hasDiagnostic(result.Diagnostics, test.rule, manifestv1alpha1.DiagnosticSeverity_DIAGNOSTIC_SEVERITY_ERROR) {
				t.Fatalf("Normalize() diagnostics = %s, want %s error", diagnosticSummary(result.Diagnostics), test.rule)
			}
		})
	}
}

func completeServiceDiscoveryTiming() *optionsv1alpha1.ServiceDiscoveryTiming {
	return &optionsv1alpha1.ServiceDiscoveryTiming{
		LiveTtlSeconds:       10,
		InitialDelay:         &optionsv1alpha1.DurationRange{Minimum: durationpb.New(0), Maximum: durationpb.New(0)},
		RepetitionsBaseDelay: durationpb.New(time.Millisecond),
		RepetitionsMax:       1,
		RequestResponseDelay: &optionsv1alpha1.DurationRange{Minimum: durationpb.New(0), Maximum: durationpb.New(time.Millisecond)},
		CyclicOfferDelay:     durationpb.New(time.Millisecond),
	}
}

func tpDirection(
	analysis *manifestv1alpha1.ProviderMemberTpAnalysisManifest,
	direction manifestv1alpha1.MemberDirection,
) *manifestv1alpha1.MemberDirectionTpAnalysisManifest {
	for _, candidate := range analysis.GetDirections() {
		if candidate.GetDirection() == direction {
			return candidate
		}
	}
	return nil
}

func loadSDTPFixtureSet(t *testing.T) *descriptor.Set {
	t.Helper()
	input := fixtureDescriptorSet(t, testv1alpha1.File_io_github_someip_proto_test_v1alpha1_sd_tp_schema_fixture_proto)
	set, err := descriptor.Load(input, descriptor.DefaultLimits())
	if err != nil {
		t.Fatalf("descriptor.Load() error = %v", err)
	}
	return set
}

func mutableSDTPFixtureDescriptorSet(t *testing.T) *descriptorpb.FileDescriptorSet {
	t.Helper()
	input := fixtureDescriptorSet(t, testv1alpha1.File_io_github_someip_proto_test_v1alpha1_sd_tp_schema_fixture_proto)
	set := &descriptorpb.FileDescriptorSet{}
	if err := proto.Unmarshal(input, set); err != nil {
		t.Fatalf("unmarshal SD/TP fixture descriptor set: %v", err)
	}
	return set
}

func providerBindingByName(
	t *testing.T,
	profile *manifestv1alpha1.ProfileManifest,
	providerName string,
) *manifestv1alpha1.ProviderServiceVersionBindingManifest {
	t.Helper()
	for _, provider := range profile.GetProviderInstances() {
		if provider.GetInstance().GetProviderName() != providerName {
			continue
		}
		if len(provider.GetServiceVersionBindings()) == 1 {
			return provider.GetServiceVersionBindings()[0]
		}
	}
	t.Fatalf("provider binding for %q was not found", providerName)
	return nil
}

func consumerBindingByName(
	t *testing.T,
	profile *manifestv1alpha1.ProfileManifest,
	consumerName string,
) *manifestv1alpha1.ConsumerServiceBindingManifest {
	t.Helper()
	for _, consumer := range profile.GetConsumerBindings() {
		if consumer.GetBinding().GetConsumerName() == consumerName {
			return consumer
		}
	}
	t.Fatalf("consumer binding %q was not found", consumerName)
	return nil
}

func memberTPAnalysisByID(
	t *testing.T,
	analyses []*manifestv1alpha1.ProviderMemberTpAnalysisManifest,
	memberID uint32,
) *manifestv1alpha1.ProviderMemberTpAnalysisManifest {
	t.Helper()
	for _, analysis := range analyses {
		if analysis.GetMemberId() == memberID {
			return analysis
		}
	}
	t.Fatalf("member TP analysis %d was not found", memberID)
	return nil
}

func assertTPDirectionResult(
	t *testing.T,
	binding *manifestv1alpha1.ProviderServiceVersionBindingManifest,
	memberID uint32,
	direction manifestv1alpha1.MemberDirection,
	want manifestv1alpha1.TpUseResult,
	wantRuntimeTP bool,
) {
	t.Helper()
	analysis := tpDirection(memberTPAnalysisByID(t, binding.GetMemberTpAnalysis(), memberID), direction)
	if analysis == nil || analysis.GetTpUseResult() != want || analysis.GetRuntimeTpSessionHandlingRequired() != wantRuntimeTP {
		t.Fatalf("member %d direction %s analysis = %#v, want result=%s runtime=%t", memberID, direction, analysis, want, wantRuntimeTP)
	}
}
