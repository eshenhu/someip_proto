package render

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	manifestv1alpha1 "github.com/eshenhu/someip_proto/gen/go/io/github/someip_proto/manifest/v1alpha1"
	optionsv1alpha1 "github.com/eshenhu/someip_proto/gen/go/io/github/someip_proto/options/v1alpha1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func TestRender_deterministicArtifacts(t *testing.T) {
	manifest := testManifest()

	firstBinary, err := Binary(manifest)
	if err != nil {
		t.Fatalf("Binary() error = %v", err)
	}
	secondBinary, err := Binary(manifest)
	if err != nil {
		t.Fatalf("Binary() second error = %v", err)
	}
	if !bytes.Equal(firstBinary, secondBinary) {
		t.Fatal("Binary() output is not deterministic")
	}
	decoded := &manifestv1alpha1.Manifest{}
	if err := proto.Unmarshal(firstBinary, decoded); err != nil {
		t.Fatalf("unmarshal Binary() output: %v", err)
	}
	if !proto.Equal(manifest, decoded) {
		t.Fatal("Binary() round trip changed the manifest")
	}

	firstJSON, err := JSON(manifest)
	if err != nil {
		t.Fatalf("JSON() error = %v", err)
	}
	secondJSON, err := JSON(manifest)
	if err != nil {
		t.Fatalf("JSON() second error = %v", err)
	}
	if !bytes.Equal(firstJSON, secondJSON) {
		t.Fatal("JSON() output is not deterministic")
	}
	if !bytes.Contains(firstJSON, []byte(`"profile_name"`)) || !bytes.HasSuffix(firstJSON, []byte("\n")) {
		t.Fatalf("JSON() output = %q, want snake_case canonical JSON ending in a newline", firstJSON)
	}

	firstMarkdown, err := Markdown(manifest)
	if err != nil {
		t.Fatalf("Markdown() error = %v", err)
	}
	secondMarkdown, err := Markdown(manifest)
	if err != nil {
		t.Fatalf("Markdown() second error = %v", err)
	}
	if !bytes.Equal(firstMarkdown, secondMarkdown) {
		t.Fatal("Markdown() output is not deterministic")
	}
	if !bytes.Contains(firstMarkdown, []byte("SIP-PROFILE-001")) ||
		!bytes.Contains(firstMarkdown, []byte("Enum types")) ||
		!bytes.Contains(firstMarkdown, []byte("Encoded field (base64): `CCo=`")) ||
		!bytes.Contains(firstMarkdown, []byte("Authority: `RULE_AUTHORITY_NORMATIVE`")) ||
		!bytes.Contains(firstMarkdown, []byte("Remediation: repair it")) ||
		!bytes.Contains(firstMarkdown, []byte("Documentation URIs: `https://example.test/docs`")) ||
		!bytes.Contains(firstMarkdown, []byte("Static Service Discovery offer")) ||
		!bytes.Contains(firstMarkdown, []byte("Derived provider TP analysis")) ||
		!bytes.Contains(firstMarkdown, []byte("Consumer bindings")) ||
		!bytes.Contains(firstMarkdown, []byte("Inherited provider TP analysis")) ||
		!bytes.Contains(firstMarkdown, []byte("Directional payload bounds")) {
		t.Fatalf("Markdown() output = %q, want diagnostics, core facts, and static SD/TP evidence", firstMarkdown)
	}

	firstHTML, err := HTML(manifest)
	if err != nil {
		t.Fatalf("HTML() error = %v", err)
	}
	secondHTML, err := HTML(manifest)
	if err != nil {
		t.Fatalf("HTML() second error = %v", err)
	}
	if !bytes.Equal(firstHTML, secondHTML) {
		t.Fatal("HTML() output is not deterministic")
	}
	if bytes.Contains(firstHTML, []byte("<script>")) || !bytes.Contains(firstHTML, []byte("&lt;script&gt;")) {
		t.Fatalf("HTML() did not safely escape descriptor-derived content: %s", firstHTML)
	}
	if bytes.Contains(firstHTML, []byte("<script src")) || bytes.Contains(firstHTML, []byte("<a href=")) || !bytes.Contains(firstHTML, []byte("https://example.test/docs")) {
		t.Fatalf("HTML() contains an unsafe link or omits URI evidence: %s", firstHTML)
	}
	for _, marker := range []string{
		`<div class="summary-card error">`,
		`<section id="diagnostics">`,
		`<div class="findings"><table>`,
		`<span class="badge">Error</span>`,
		`<th>Remediation</th>`,
		`<h5>Static SD/TP contract:`,
		`<h3>Consumer bindings</h3>`,
		`Derived provider TP analysis`,
		`Inherited provider TP analysis`,
	} {
		if !bytes.Contains(firstHTML, []byte(marker)) {
			t.Fatalf("HTML() missing structured report marker %q", marker)
		}
	}
	if bytes.Contains(firstHTML, []byte("<pre>")) {
		t.Fatal("HTML() still wraps the Markdown report in a preformatted block")
	}
}

func TestJSONDiagnostics_versionedEnvelope(t *testing.T) {
	diagnostics := testManifest().GetDiagnostics()
	encoded, err := JSONDiagnostics(diagnostics)
	if err != nil {
		t.Fatalf("JSONDiagnostics() error = %v", err)
	}
	report := &manifestv1alpha1.DiagnosticReport{}
	if err := protojson.Unmarshal(encoded, report); err != nil {
		t.Fatalf("unmarshal JSON diagnostics: %v", err)
	}
	if report.GetDiagnosticFormatVersion() != 1 || len(report.GetDiagnostics()) != len(diagnostics) {
		t.Fatalf("JSONDiagnostics() report = %#v, want versioned diagnostic envelope", report)
	}
	if text := TextDiagnostics(diagnostics); !bytes.Contains(text, []byte("remediation: repair it")) {
		t.Fatalf("TextDiagnostics() output = %q, want remediation", text)
	}
}

func TestWriteFileAtomic_replacesCompleteArtifact(t *testing.T) {
	path := filepath.Join(t.TempDir(), "manifest.json")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatalf("write prior artifact: %v", err)
	}
	if err := WriteFileAtomic(path, []byte("new"), 0o600); err != nil {
		t.Fatalf("WriteFileAtomic() error = %v", err)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read artifact: %v", err)
	}
	if string(contents) != "new" {
		t.Fatalf("artifact contents = %q, want new", contents)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatalf("read artifact directory: %v", err)
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), ".tmp-") {
			t.Fatalf("temporary artifact %q was not cleaned up", entry.Name())
		}
	}
}

func testManifest() *manifestv1alpha1.Manifest {
	return &manifestv1alpha1.Manifest{
		Provenance: &manifestv1alpha1.ManifestProvenance{
			ManifestFormatVersion: 1,
			OptionsApiVersion:     "v1alpha1",
			InputFdsSha256:        "abc123",
			ToolVersion:           "test",
			NormativeSourceCommit: "pinned",
		},
		Profile: &manifestv1alpha1.ProfileManifest{
			ProfileName:       "fixture.<script>alert(1)</script>",
			Version:           &optionsv1alpha1.SemanticVersion{Major: 1, Minor: 2, Patch: 3},
			DocumentationUris: []string{"https://example.test/docs"},
			ProviderInstances: []*manifestv1alpha1.ProviderInstanceManifest{{
				Instance: &optionsv1alpha1.ProviderInstance{
					ServiceId:           100,
					InstanceId:          1,
					ProviderName:        "fixture.provider",
					DefaultEndpointName: "provider-udp",
					ServiceLineage:      &optionsv1alpha1.ServiceLineage{Id: "fixture.lineage"},
					Endpoints: []*optionsv1alpha1.Endpoint{{
						Name:      "provider-udp",
						Address:   "192.0.2.10",
						Transport: optionsv1alpha1.TransportProtocol_TRANSPORT_PROTOCOL_UDP,
						Port:      30500,
					}},
				},
				ServiceVersionBindings: []*manifestv1alpha1.ProviderServiceVersionBindingManifest{{
					Binding: &optionsv1alpha1.ServiceVersionBinding{
						Service:               &optionsv1alpha1.ProtobufReference{Fqn: ".fixture.Service"},
						InterfaceMajorVersion: 1,
					},
					ServiceLineage: &optionsv1alpha1.ServiceLineage{Id: "fixture.lineage"},
					ServiceDiscoveryOffer: &optionsv1alpha1.ServiceDiscoveryOffer{
						EndpointNames: []string{"provider-udp"},
						Timing:        &optionsv1alpha1.ServiceDiscoveryTiming{LiveTtlSeconds: 10},
						EventGroupDeliveryPolicies: []*optionsv1alpha1.EventGroupDeliveryPolicy{{
							EventGroupName: "updates",
							Mode:           optionsv1alpha1.EventGroupDeliveryMode_EVENT_GROUP_DELIVERY_MODE_UNICAST,
						}},
					},
					EventGroupDeliveryPolicies: []*optionsv1alpha1.EventGroupDeliveryPolicy{{
						EventGroupName: "updates",
						Mode:           optionsv1alpha1.EventGroupDeliveryMode_EVENT_GROUP_DELIVERY_MODE_UNICAST,
					}},
					MemberTpConfigurations: []*optionsv1alpha1.MemberTpConfiguration{{MemberId: 1, TpWhenNeeded: true}},
					MemberTpAnalysis:       []*manifestv1alpha1.ProviderMemberTpAnalysisManifest{testTPAnalysis("provider-udp", "192.0.2.10")},
				}},
			}},
			ConsumerBindings: []*manifestv1alpha1.ConsumerServiceBindingManifest{{
				Binding: &optionsv1alpha1.ConsumerServiceBinding{
					ConsumerName:          "fixture.consumer",
					Service:               &optionsv1alpha1.ProtobufReference{Fqn: ".fixture.Service"},
					ServiceLineage:        &optionsv1alpha1.ServiceLineage{Id: "fixture.lineage"},
					InterfaceMajorVersion: 1,
					DefaultEndpointName:   "consumer-udp",
				},
				ProviderSelector: &manifestv1alpha1.ResolvedProviderSelectorManifest{
					Selector: &optionsv1alpha1.ProviderSelector{Selector: &optionsv1alpha1.ProviderSelector_NamedProviderName{NamedProviderName: "fixture.provider"}},
					Candidates: []*manifestv1alpha1.ProviderCandidateManifest{{
						ProviderName:          "fixture.provider",
						ServiceId:             100,
						InstanceId:            1,
						Service:               &optionsv1alpha1.ProtobufReference{Fqn: ".fixture.Service"},
						InterfaceMajorVersion: 1,
						ServiceLineage:        &optionsv1alpha1.ServiceLineage{Id: "fixture.lineage"},
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
					Subscription:         &optionsv1alpha1.Subscription{EventGroupName: "updates", TtlSeconds: 5, LocalEndpointNames: []string{"consumer-udp"}},
					EventGroupId:         1,
					DeliveryPolicy:       &optionsv1alpha1.EventGroupDeliveryPolicy{EventGroupName: "updates", Mode: optionsv1alpha1.EventGroupDeliveryMode_EVENT_GROUP_DELIVERY_MODE_UNICAST},
					InitialDataRequested: false,
				}},
				InheritedMemberTpAnalysis: []*manifestv1alpha1.ProviderMemberTpAnalysisManifest{testTPAnalysis("consumer-udp", "192.0.2.20")},
			}},
		},
		Services: []*manifestv1alpha1.ServiceManifest{{
			Service:        &optionsv1alpha1.ProtobufReference{Fqn: ".fixture.Service"},
			ServiceLineage: &optionsv1alpha1.ServiceLineage{Id: "fixture.lineage"},
			Members: []*manifestv1alpha1.ServiceMemberManifest{{
				Name:     "Get",
				MemberId: 1,
				Kind:     "method",
				DirectionalPayloads: []*manifestv1alpha1.MemberDirectionalPayloadManifest{{
					Direction:   manifestv1alpha1.MemberDirection_MEMBER_DIRECTION_REQUEST,
					EncodedSize: &manifestv1alpha1.EncodedSize{ExactBytes: 32},
				}},
			}},
		}},
		Diagnostics: []*manifestv1alpha1.Diagnostic{{
			RuleCode:           "SIP-PROFILE-001",
			Severity:           manifestv1alpha1.DiagnosticSeverity_DIAGNOSTIC_SEVERITY_ERROR,
			Authority:          manifestv1alpha1.RuleAuthority_RULE_AUTHORITY_NORMATIVE,
			AuthorityReference: "src/someip-rpc.rst#example",
			Location:           &manifestv1alpha1.SourceLocation{ImportPath: "fixture.proto", Line: 1, Column: 1},
			Remediation:        "repair it",
			Message:            "descriptor-derived <script>alert(1)</script>",
		}},
		EnumTypes: []*manifestv1alpha1.EnumManifest{{
			Type:    &optionsv1alpha1.ProtobufReference{Fqn: ".fixture.Mode"},
			Options: &optionsv1alpha1.EnumOptionsSpec{WireWidthBits: 8},
			Values:  []*manifestv1alpha1.EnumValueManifest{{Name: "MODE_ENABLED", Number: 1}},
		}},
		UnknownOptions: []*manifestv1alpha1.UnknownOption{{
			ExtendeeKind: manifestv1alpha1.OptionExtendeeKind_OPTION_EXTENDEE_KIND_FILE,
			FieldNumber:  1,
			RawValue:     []byte{42},
			WireType:     0,
			EncodedField: []byte{8, 42},
		}},
	}
}

func testTPAnalysis(endpointName, address string) *manifestv1alpha1.ProviderMemberTpAnalysisManifest {
	return &manifestv1alpha1.ProviderMemberTpAnalysisManifest{
		MemberId:     1,
		TpWhenNeeded: true,
		Directions: []*manifestv1alpha1.MemberDirectionTpAnalysisManifest{{
			Direction:                        manifestv1alpha1.MemberDirection_MEMBER_DIRECTION_REQUEST,
			EncodedSize:                      &manifestv1alpha1.EncodedSize{MinimumBytes: 0, MaximumBytes: 1600},
			EffectiveTransport:               manifestv1alpha1.EffectiveTransportClassification_EFFECTIVE_TRANSPORT_CLASSIFICATION_UDP,
			Endpoint:                         &manifestv1alpha1.EffectiveEndpointManifest{EndpointName: endpointName, Address: address, Transport: optionsv1alpha1.TransportProtocol_TRANSPORT_PROTOCOL_UDP, Port: &manifestv1alpha1.EffectiveEndpointManifest_FixedPort{FixedPort: 30500}},
			TpWhenNeeded:                     true,
			UnsegmentedUdpPayloadBudgetBytes: 1400,
			TpUseResult:                      manifestv1alpha1.TpUseResult_TP_USE_RESULT_MAY_BE_NEEDED,
			RuntimeTpSessionHandlingRequired: true,
		}},
	}
}
