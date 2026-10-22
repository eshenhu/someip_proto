package semantic

import (
	"bytes"
	"testing"

	manifestv1alpha1 "github.com/eshenhu/someip_proto/gen/go/io/github/someip_proto/manifest/v1alpha1"
	optionsv1alpha1 "github.com/eshenhu/someip_proto/gen/go/io/github/someip_proto/options/v1alpha1"
	"github.com/eshenhu/someip_proto/internal/render"
	"google.golang.org/protobuf/proto"
)

func TestCanonicalOptions_normalizesNonSemanticSequenceOrder(t *testing.T) {
	providerA := &optionsv1alpha1.ProviderInstance{
		Endpoints: []*optionsv1alpha1.Endpoint{
			{Name: "z", Address: "192.0.2.2", Transport: optionsv1alpha1.TransportProtocol_TRANSPORT_PROTOCOL_TCP, Port: 2},
			{Name: "a", Address: "192.0.2.1", Transport: optionsv1alpha1.TransportProtocol_TRANSPORT_PROTOCOL_UDP, Port: 1},
		},
		MemberEndpointOverrides: []*optionsv1alpha1.MemberEndpointOverride{
			{MemberId: 2, EndpointName: "z"},
			{MemberId: 1, EndpointName: "a"},
		},
		ServiceVersionBindings: []*optionsv1alpha1.ServiceVersionBinding{
			{Service: &optionsv1alpha1.ProtobufReference{Fqn: ".example.V2"}, InterfaceMajorVersion: 2, MemberEndpointOverrides: []*optionsv1alpha1.MemberEndpointOverride{{MemberId: 2, EndpointName: "z"}, {MemberId: 1, EndpointName: "a"}}},
			{Service: &optionsv1alpha1.ProtobufReference{Fqn: ".example.V1"}, InterfaceMajorVersion: 1, MemberEndpointOverrides: []*optionsv1alpha1.MemberEndpointOverride{{MemberId: 2, EndpointName: "z"}, {MemberId: 1, EndpointName: "a"}}},
		},
	}
	providerB := proto.Clone(providerA).(*optionsv1alpha1.ProviderInstance)
	reverse(providerB.Endpoints)
	reverse(providerB.MemberEndpointOverrides)
	reverse(providerB.ServiceVersionBindings)
	for _, binding := range providerB.ServiceVersionBindings {
		reverse(binding.MemberEndpointOverrides)
	}
	if !proto.Equal(canonicalProviderInstance(providerA), canonicalProviderInstance(providerB)) {
		t.Fatal("canonicalProviderInstance() retained a nonsemantic declaration order")
	}

	serviceA := &optionsv1alpha1.ServiceOptionsSpec{
		DocumentationUris: []string{"https://example.test/z", "https://example.test/a"},
		EventGroups:       []*optionsv1alpha1.EventGroup{{Id: 2, Name: "z"}, {Id: 1, Name: "a"}},
		Events: []*optionsv1alpha1.Event{
			{Id: 2, Name: "z", EventGroupNames: []string{"z", "a"}},
			{Id: 1, Name: "a", EventGroupNames: []string{"z", "a"}},
		},
		Fields: []*optionsv1alpha1.ServiceField{
			{Name: "z", EventGroupNames: []string{"z", "a"}},
			{Name: "a", EventGroupNames: []string{"z", "a"}},
		},
	}
	serviceB := proto.Clone(serviceA).(*optionsv1alpha1.ServiceOptionsSpec)
	reverse(serviceB.DocumentationUris)
	reverse(serviceB.EventGroups)
	reverse(serviceB.Events)
	reverse(serviceB.Fields)
	for _, event := range serviceB.Events {
		reverse(event.EventGroupNames)
	}
	for _, field := range serviceB.Fields {
		reverse(field.EventGroupNames)
	}
	if !proto.Equal(canonicalServiceOptions(serviceA), canonicalServiceOptions(serviceB)) {
		t.Fatal("canonicalServiceOptions() retained a nonsemantic declaration order")
	}

	methodA := &optionsv1alpha1.MethodOptionsSpec{
		DocumentationUris: []string{"https://example.test/z", "https://example.test/a"},
		ReturnCodes:       []*optionsv1alpha1.ReturnCode{{Code: 2, Name: "Z"}, {Code: 1, Name: "A"}},
	}
	methodB := proto.Clone(methodA).(*optionsv1alpha1.MethodOptionsSpec)
	reverse(methodB.DocumentationUris)
	reverse(methodB.ReturnCodes)
	if !proto.Equal(canonicalMethodOptions(methodA), canonicalMethodOptions(methodB)) {
		t.Fatal("canonicalMethodOptions() retained a nonsemantic declaration order")
	}

	messageA := &optionsv1alpha1.MessageOptionsSpec{
		DocumentationUris: []string{"https://example.test/z", "https://example.test/a"},
		StructLayout: &optionsv1alpha1.StructLayout{Padding: []*optionsv1alpha1.Padding{
			{Position: &optionsv1alpha1.Padding_AtEnd{AtEnd: true}, ByteCount: 3},
			{Position: &optionsv1alpha1.Padding_BeforeWireOrder{BeforeWireOrder: 2}, ByteCount: 2},
			{Position: &optionsv1alpha1.Padding_BeforeWireOrder{BeforeWireOrder: 1}, ByteCount: 1},
		}},
	}
	messageB := proto.Clone(messageA).(*optionsv1alpha1.MessageOptionsSpec)
	reverse(messageB.DocumentationUris)
	reverse(messageB.StructLayout.Padding)
	if !proto.Equal(canonicalMessageOptions(messageA), canonicalMessageOptions(messageB)) {
		t.Fatal("canonicalMessageOptions() retained a nonsemantic declaration order")
	}

	fieldA := &optionsv1alpha1.FieldOptionsSpec{
		DocumentationUris: []string{"https://example.test/z", "https://example.test/a"},
		Bitfield: &optionsv1alpha1.BitfieldLayout{NamedBits: []*optionsv1alpha1.NamedBit{
			{BitIndex: 2, Name: "z"},
			{BitIndex: 1, Name: "a"},
		}},
	}
	fieldB := proto.Clone(fieldA).(*optionsv1alpha1.FieldOptionsSpec)
	reverse(fieldB.DocumentationUris)
	reverse(fieldB.Bitfield.NamedBits)
	if !proto.Equal(canonicalFieldOptions(fieldA), canonicalFieldOptions(fieldB)) {
		t.Fatal("canonicalFieldOptions() retained a nonsemantic declaration order")
	}
}

func TestNormalize_canonicalizesFDSFileOrderExceptRawInputProvenance(t *testing.T) {
	baseline := mutableFixtureDescriptorSet(t)
	permuted := mutableFixtureDescriptorSet(t)
	reverse(permuted.File)

	baselineResult := normalizeMutableFixture(t, baseline)
	permutedResult := normalizeMutableFixture(t, permuted)
	if baselineResult.HasErrors() || permutedResult.HasErrors() {
		t.Fatalf("Normalize() errors = %s / %s", diagnosticSummary(baselineResult.Diagnostics), diagnosticSummary(permutedResult.Diagnostics))
	}
	if baselineResult.Manifest.GetProvenance().GetInputFdsSha256() == permutedResult.Manifest.GetProvenance().GetInputFdsSha256() {
		t.Fatal("permuted FileDescriptorSet unexpectedly retained its raw input digest")
	}

	baselineManifest := proto.Clone(baselineResult.Manifest).(*manifestv1alpha1.Manifest)
	permutedManifest := proto.Clone(permutedResult.Manifest).(*manifestv1alpha1.Manifest)
	baselineManifest.Provenance.InputFdsSha256 = ""
	permutedManifest.Provenance.InputFdsSha256 = ""
	if !proto.Equal(baselineManifest, permutedManifest) {
		t.Fatal("normalization retained FileDescriptorSet declaration order")
	}

	formats := []struct {
		name   string
		render func(*manifestv1alpha1.Manifest) ([]byte, error)
	}{
		{name: "binary", render: render.Binary},
		{name: "json", render: render.JSON},
		{name: "markdown", render: render.Markdown},
		{name: "html", render: render.HTML},
	}
	for _, format := range formats {
		t.Run(format.name, func(t *testing.T) {
			left, err := format.render(baselineManifest)
			if err != nil {
				t.Fatalf("render baseline: %v", err)
			}
			right, err := format.render(permutedManifest)
			if err != nil {
				t.Fatalf("render permuted: %v", err)
			}
			if !bytes.Equal(left, right) {
				t.Fatalf("%s rendering retained FileDescriptorSet declaration order", format.name)
			}
		})
	}
}

func TestNormalize_canonicalizesStaticSDTPSequenceOrder(t *testing.T) {
	baseline := mutableSDTPFixtureDescriptorSet(t)
	permuted := mutableSDTPFixtureDescriptorSet(t)
	file := fixtureFile(t, permuted, "io/github/someip_proto/test/v1alpha1/sd_tp_schema_fixture.proto")
	profile := deploymentProfileOptions(t, file)
	reverse(profile.ProviderInstances)
	reverse(profile.ConsumerBindings)
	for _, provider := range profile.GetProviderInstances() {
		reverse(provider.Endpoints)
		reverse(provider.ServiceVersionBindings)
		for _, binding := range provider.GetServiceVersionBindings() {
			reverse(binding.MemberEndpointOverrides)
			reverse(binding.MemberTpConfigurations)
			if offer := binding.GetServiceDiscoveryOffer(); offer != nil {
				reverse(offer.EndpointNames)
				reverse(offer.EventGroupDeliveryPolicies)
			}
		}
	}
	for _, consumer := range profile.GetConsumerBindings() {
		reverse(consumer.LocalEndpoints)
		reverse(consumer.MemberEndpointOverrides)
		reverse(consumer.Subscriptions)
		for _, subscription := range consumer.GetSubscriptions() {
			reverse(subscription.LocalEndpointNames)
		}
	}

	baselineResult := normalizeMutableFixtureForProfile(t, baseline, "sd-tp-valid")
	permutedResult := normalizeMutableFixtureForProfile(t, permuted, "sd-tp-valid")
	if baselineResult.HasErrors() || permutedResult.HasErrors() {
		t.Fatalf("Normalize() errors = %s / %s", diagnosticSummary(baselineResult.Diagnostics), diagnosticSummary(permutedResult.Diagnostics))
	}
	baselineManifest := proto.Clone(baselineResult.Manifest).(*manifestv1alpha1.Manifest)
	permutedManifest := proto.Clone(permutedResult.Manifest).(*manifestv1alpha1.Manifest)
	baselineManifest.Provenance.InputFdsSha256 = ""
	permutedManifest.Provenance.InputFdsSha256 = ""
	if !proto.Equal(baselineManifest, permutedManifest) {
		t.Fatal("normalization retained static SD/TP declaration order")
	}
	for _, format := range []struct {
		name   string
		render func(*manifestv1alpha1.Manifest) ([]byte, error)
	}{
		{name: "binary", render: render.Binary},
		{name: "json", render: render.JSON},
		{name: "markdown", render: render.Markdown},
		{name: "html", render: render.HTML},
	} {
		t.Run(format.name, func(t *testing.T) {
			left, err := format.render(baselineManifest)
			if err != nil {
				t.Fatalf("render baseline: %v", err)
			}
			right, err := format.render(permutedManifest)
			if err != nil {
				t.Fatalf("render permuted: %v", err)
			}
			if !bytes.Equal(left, right) {
				t.Fatalf("%s rendering retained static SD/TP declaration order", format.name)
			}
		})
	}
}

func reverse[T any](values []T) {
	for left, right := 0, len(values)-1; left < right; left, right = left+1, right-1 {
		values[left], values[right] = values[right], values[left]
	}
}
