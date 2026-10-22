package semantic

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"testing"

	manifestv1alpha1 "github.com/eshenhu/someip_proto/gen/go/io/github/someip_proto/manifest/v1alpha1"
	optionsv1alpha1 "github.com/eshenhu/someip_proto/gen/go/io/github/someip_proto/options/v1alpha1"
	testv1alpha1 "github.com/eshenhu/someip_proto/gen/go/io/github/someip_proto/test/v1alpha1"
	"github.com/eshenhu/someip_proto/internal/descriptor"
	"github.com/eshenhu/someip_proto/internal/testutil"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/known/emptypb"
)

func TestNormalize_phaseTwoOneValidProfile(t *testing.T) {
	set := loadFixtureSet(t)

	result := Normalize(set, "phase-2-1-valid", DefaultConfig())
	if result.HasErrors() {
		t.Fatalf("Normalize() errors = %s", diagnosticSummary(result.Diagnostics))
	}
	if result.Manifest.GetProfile().GetProfileName() != "phase-2-1-valid" {
		t.Fatalf("profile name = %q, want phase-2-1-valid", result.Manifest.GetProfile().GetProfileName())
	}
	providers := result.Manifest.GetProfile().GetProviderInstances()
	if len(providers) != 1 || len(providers[0].GetServiceVersionBindings()) != 2 {
		t.Fatalf("provider bindings = %#v, want one provider with two service-major bindings", providers)
	}
	if !hasDiagnostic(result.Diagnostics, "SIP-UNION-002", manifestv1alpha1.DiagnosticSeverity_DIAGNOSTIC_SEVERITY_WARNING) {
		t.Fatalf("diagnostics = %s, want zero-width union warning", diagnosticSummary(result.Diagnostics))
	}
	if !hasDiagnostic(result.Diagnostics, "SIP-LAYOUT-003", manifestv1alpha1.DiagnosticSeverity_DIAGNOSTIC_SEVERITY_WARNING) {
		t.Fatalf("diagnostics = %s, want alignment warning", diagnosticSummary(result.Diagnostics))
	}

	union := findUnion(t, result.Manifest, ".io.github.someip_proto.test.v1alpha1.UnequalFixedZeroWidthUnion")
	if union.GetEffectiveValueSlotBytes() != 2 {
		t.Fatalf("effective union slot = %d, want 2", union.GetEffectiveValueSlotBytes())
	}
	if got := union.GetAlternatives()[0].GetFixedDerivedZeroFillBytes(); got != 1 {
		t.Fatalf("first union alternative derived zero fill = %d, want 1", got)
	}

	dynamicUnion := findUnion(t, result.Manifest, ".io.github.someip_proto.test.v1alpha1.BoundedDynamicZeroWidthUnion")
	if dynamicUnion.GetEffectiveValueSlotBytes() != 8 {
		t.Fatalf("dynamic union effective slot = %d, want 8", dynamicUnion.GetEffectiveValueSlotBytes())
	}
	if !dynamicUnion.GetAlternatives()[0].GetPadToEffectiveSlot() {
		t.Fatal("dynamic union alternative must use data-dependent zero fill")
	}
	if layout := dynamicUnion.GetAlternatives()[0].GetEffectiveLayout(); layout.GetByteOrder() != optionsv1alpha1.ByteOrder_BYTE_ORDER_BIG_ENDIAN || layout.GetLengthFieldWidth() != optionsv1alpha1.LengthFieldWidth_LENGTH_FIELD_WIDTH_8 {
		t.Fatalf("dynamic union alternative effective layout = %#v, want resolved big-endian 8-bit framing", layout)
	}

	paddedUnion := findUnion(t, result.Manifest, ".io.github.someip_proto.test.v1alpha1.LengthPrefixedUnionPadding")
	if got := paddedUnion.GetAlternatives()[0].GetDeclaredPaddingBytes(); got != 3 {
		t.Fatalf("declared union padding = %d, want 3", got)
	}
	if got := paddedUnion.GetEncodedSize().GetMaximumBytes(); got != 7 {
		t.Fatalf("length-prefixed union maximum size = %d, want 7", got)
	}

	service := findService(t, result.Manifest, ".io.github.someip_proto.test.v1alpha1.PhaseTwoOneV1Service")
	if members := service.GetMembers(); len(members) != 1 || members[0].GetMethodOptions().GetResponsePolicy() != optionsv1alpha1.ResponsePolicy_RESPONSE_POLICY_RESPONSE_ONLY {
		t.Fatalf("service method contract = %#v, want normalized request/response metadata", members)
	}
	enum := findEnum(t, result.Manifest, ".io.github.someip_proto.test.v1alpha1.PhaseTwoOneMode")
	if enum.GetOptions().GetWireWidthBits() != 8 || len(enum.GetValues()) != 2 || !enum.GetValues()[0].GetOptions().GetNonSendablePlaceholder() {
		t.Fatalf("enum manifest = %#v, want wire width and normalized values", enum)
	}
}

func TestNormalize_phaseTwoOneInvalidProfile(t *testing.T) {
	set := loadFixtureSet(t)

	result := Normalize(set, "phase-2-1-invalid", DefaultConfig())
	if !result.HasErrors() {
		t.Fatalf("Normalize() diagnostics = %s, want profile errors", diagnosticSummary(result.Diagnostics))
	}
	if !hasDiagnostic(result.Diagnostics, "SIP-PROFILE-001", manifestv1alpha1.DiagnosticSeverity_DIAGNOSTIC_SEVERITY_ERROR) {
		t.Fatalf("diagnostics = %s, want profile validation error", diagnosticSummary(result.Diagnostics))
	}
}

func TestNormalize_materializesDocumentationURIs(t *testing.T) {
	set := mutableFixtureDescriptorSet(t)
	file := fixtureFile(t, set, "io/github/someip_proto/test/v1alpha1/phase_2_1_schema_fixture.proto")
	fileOptions, ok := proto.GetExtension(file.GetOptions(), optionsv1alpha1.E_File).(*optionsv1alpha1.FileOptionsSpec)
	if !ok || fileOptions == nil {
		t.Fatal("fixture file lacks project options")
	}
	fileOptions = proto.Clone(fileOptions).(*optionsv1alpha1.FileOptionsSpec)
	fileOptions.DocumentationUris = []string{"https://example.test/profile"}
	proto.SetExtension(file.GetOptions(), optionsv1alpha1.E_File, fileOptions)

	service := fixtureService(t, set, "PhaseTwoOneV1Service")
	serviceOptions, ok := proto.GetExtension(service.GetOptions(), optionsv1alpha1.E_Service).(*optionsv1alpha1.ServiceOptionsSpec)
	if !ok || serviceOptions == nil {
		t.Fatal("fixture service lacks project options")
	}
	serviceOptions = proto.Clone(serviceOptions).(*optionsv1alpha1.ServiceOptionsSpec)
	serviceOptions.DocumentationUris = []string{"mailto:owners@example.test"}
	proto.SetExtension(service.GetOptions(), optionsv1alpha1.E_Service, serviceOptions)

	result := normalizeMutableFixture(t, set)
	if result.HasErrors() {
		t.Fatalf("Normalize() diagnostics = %s, want valid documentation URIs", diagnosticSummary(result.Diagnostics))
	}
	if got := result.Manifest.GetProfile().GetDocumentationUris(); len(got) != 1 || got[0] != "https://example.test/profile" {
		t.Fatalf("profile documentation URIs = %#v, want materialized URI", got)
	}
	if got := findService(t, result.Manifest, ".io.github.someip_proto.test.v1alpha1.PhaseTwoOneV1Service").GetDocumentationUris(); len(got) != 1 || got[0] != "mailto:owners@example.test" {
		t.Fatalf("service documentation URIs = %#v, want materialized URI", got)
	}
}

func TestNormalize_rejectsUnsafeDocumentationURI(t *testing.T) {
	set := mutableFixtureDescriptorSet(t)
	service := fixtureService(t, set, "PhaseTwoOneV1Service")
	options, ok := proto.GetExtension(service.GetOptions(), optionsv1alpha1.E_Service).(*optionsv1alpha1.ServiceOptionsSpec)
	if !ok || options == nil {
		t.Fatal("fixture service lacks project options")
	}
	options = proto.Clone(options).(*optionsv1alpha1.ServiceOptionsSpec)
	options.DocumentationUris = []string{"file:///not-allowed"}
	proto.SetExtension(service.GetOptions(), optionsv1alpha1.E_Service, options)

	result := normalizeMutableFixture(t, set)
	if !hasDiagnostic(result.Diagnostics, "SIP-ARTIFACT-001", manifestv1alpha1.DiagnosticSeverity_DIAGNOSTIC_SEVERITY_ERROR) {
		t.Fatalf("Normalize() diagnostics = %s, want unsafe documentation URI error", diagnosticSummary(result.Diagnostics))
	}
}

func TestNormalize_rejectsUnsafeImportedFileDocumentationURI(t *testing.T) {
	set := mutableFixtureDescriptorSet(t)
	imported := fixtureFile(t, set, "google/protobuf/duration.proto")
	if imported.Options == nil {
		imported.Options = &descriptorpb.FileOptions{}
	}
	proto.SetExtension(imported.Options, optionsv1alpha1.E_File, &optionsv1alpha1.FileOptionsSpec{
		DocumentationUris: []string{"file:///not-allowed"},
	})

	result := normalizeMutableFixture(t, set)
	if !hasDiagnosticMessage(result.Diagnostics, "SIP-ARTIFACT-001", "google/protobuf/duration.proto") {
		t.Fatalf("Normalize() diagnostics = %s, want unsafe imported file documentation URI error", diagnosticSummary(result.Diagnostics))
	}
}

func TestNormalize_rejectsUnsafeArtifactProvenance(t *testing.T) {
	config := DefaultConfig()
	config.ToolVersion = "/tmp/local-tool"
	config.SourceRevision = `C:\work\local-revision`

	result := Normalize(loadFixtureSet(t), "phase-2-1-valid", config)
	if !hasDiagnostic(result.Diagnostics, "SIP-ARTIFACT-001", manifestv1alpha1.DiagnosticSeverity_DIAGNOSTIC_SEVERITY_ERROR) {
		t.Fatalf("Normalize() diagnostics = %s, want unsafe provenance error", diagnosticSummary(result.Diagnostics))
	}
	if provenance := result.Manifest.GetProvenance(); provenance.GetToolVersion() != "" || provenance.GetSourceRevision() != "" {
		t.Fatalf("manifest provenance = %#v, want unsafe values removed", provenance)
	}
}

func TestNormalize_rejectsUnselectedDuplicateProfileName(t *testing.T) {
	set := mutableFixtureDescriptorSet(t)
	var duplicateFile, selectedFile *descriptorpb.FileDescriptorProto
	for _, file := range set.GetFile() {
		switch file.GetName() {
		case "google/protobuf/descriptor.proto":
			duplicateFile = file
		case "google/protobuf/duration.proto":
			selectedFile = file
		}
	}
	if duplicateFile == nil || selectedFile == nil {
		t.Fatal("fixture descriptor closure lacks files needed for profile-duplicate test")
	}
	proto.SetExtension(duplicateFile.GetOptions(), optionsv1alpha1.E_File, &optionsv1alpha1.FileOptionsSpec{
		DeploymentProfile: &optionsv1alpha1.DeploymentProfile{ProfileName: "phase-2-1-valid"},
	})
	proto.SetExtension(selectedFile.GetOptions(), optionsv1alpha1.E_File, &optionsv1alpha1.FileOptionsSpec{
		DeploymentProfile: &optionsv1alpha1.DeploymentProfile{ProfileName: "unique-profile"},
	})

	result := normalizeMutableFixtureForProfile(t, set, "unique-profile")
	if !hasDiagnosticMessage(result.Diagnostics, "SIP-PROFILE-001", "declared more than once") {
		t.Fatalf("Normalize() diagnostics = %s, want unselected duplicate-profile error", diagnosticSummary(result.Diagnostics))
	}
}

func TestNormalize_rejectsImportedDeploymentProfile(t *testing.T) {
	set := mutableFixtureDescriptorSet(t)
	profileFile := fixtureFile(t, set, "io/github/someip_proto/test/v1alpha1/phase_2_1_schema_fixture.proto")
	profileFile.Dependency = append(profileFile.Dependency, "other_profile.proto")
	otherProfile := &descriptorpb.FileDescriptorProto{
		Name:       proto.String("other_profile.proto"),
		Package:    proto.String("fixture.other"),
		Syntax:     proto.String("proto3"),
		Dependency: []string{"io/github/someip_proto/options/v1alpha1/someip_options.proto"},
		Options:    &descriptorpb.FileOptions{},
		SourceCodeInfo: &descriptorpb.SourceCodeInfo{
			Location: []*descriptorpb.SourceCodeInfo_Location{{Span: []int32{0, 0, 0}}},
		},
	}
	proto.SetExtension(otherProfile.Options, optionsv1alpha1.E_File, &optionsv1alpha1.FileOptionsSpec{
		DeploymentProfile: &optionsv1alpha1.DeploymentProfile{ProfileName: "other-profile"},
	})
	set.File = append(set.File, otherProfile)

	result := normalizeMutableFixture(t, set)
	if !hasDiagnosticMessage(result.Diagnostics, "SIP-PROFILE-001", "imports another deployment profile") {
		t.Fatalf("Normalize() diagnostics = %s, want closed-profile import error", diagnosticSummary(result.Diagnostics))
	}
}

func TestValidateServiceID_reservedIdentifiers(t *testing.T) {
	for _, identifier := range []uint32{0, 0x0101, 0x433f, 0xfffe, 0xffff, 0x1_0000} {
		t.Run("identifier", func(t *testing.T) {
			collector := diagnosticCollector{}
			validateServiceID(&collector, descriptor.Location{}, identifier)
			if !hasDiagnostic(collector.diagnostics, "SIP-ID-001", manifestv1alpha1.DiagnosticSeverity_DIAGNOSTIC_SEVERITY_ERROR) {
				t.Fatalf("validateServiceID(%#x) diagnostics = %s, want SIP-ID-001 error", identifier, diagnosticSummary(collector.diagnostics))
			}
		})
	}
}

func TestNormalize_rejectsStructBodyThatExceedsPrefix(t *testing.T) {
	set := mutableFixtureDescriptorSet(t)
	message := fixtureMessage(t, set, "BoundedDynamicZeroWidthUnion")
	messageOptions := messageOptions(t, message)
	messageOptions.GetStructLayout().LengthFieldWidth = optionsv1alpha1.LengthFieldWidth_LENGTH_FIELD_WIDTH_8
	proto.SetExtension(message.GetOptions(), optionsv1alpha1.E_Message, messageOptions)

	unionOptions := oneofOptions(t, message.GetOneofDecl()[0])
	unionOptions.MaxEncodedBytes = 301
	proto.SetExtension(message.GetOneofDecl()[0].GetOptions(), optionsv1alpha1.E_Oneof, unionOptions)

	fieldOptions := fieldOptions(t, fixtureField(t, message, "dynamic_value"))
	fieldOptions.GetString_().LengthFieldWidth = optionsv1alpha1.LengthFieldWidth_LENGTH_FIELD_WIDTH_16
	fieldOptions.GetString_().MaxEncodedBytes = 300
	fieldOptions.MaxEncodedBytes = 300
	proto.SetExtension(fixtureField(t, message, "dynamic_value").GetOptions(), optionsv1alpha1.E_Field, fieldOptions)

	result := normalizeMutableFixture(t, set)
	if !hasDiagnosticMessage(result.Diagnostics, "SIP-LAYOUT-002", "struct length prefix") {
		t.Fatalf("Normalize() diagnostics = %s, want struct-prefix capacity error", diagnosticSummary(result.Diagnostics))
	}
}

func TestNormalize_requiresDynamicByteArrayMaximum(t *testing.T) {
	set := mutableFixtureDescriptorSet(t)
	message := fixtureMessage(t, set, "PhaseTwoOneV1ServiceSetRequest")
	field := fixtureField(t, message, "value")
	field.Type = descriptorpb.FieldDescriptorProto_TYPE_BYTES.Enum()
	field.TypeName = nil
	fieldOptions := fieldOptions(t, field)
	fieldOptions.ScalarRepresentation = optionsv1alpha1.ScalarRepresentation_SCALAR_REPRESENTATION_UNSPECIFIED
	fieldOptions.Array = &optionsv1alpha1.ArrayLayout{
		LengthFieldWidth: optionsv1alpha1.LengthFieldWidth_LENGTH_FIELD_WIDTH_8,
		MaxItems:         8,
	}
	fieldOptions.MaxEncodedBytes = 0
	proto.SetExtension(field.GetOptions(), optionsv1alpha1.E_Field, fieldOptions)

	result := normalizeMutableFixture(t, set)
	if !hasDiagnosticMessage(result.Diagnostics, "SIP-ARRAY-001", "max_encoded_bytes") {
		t.Fatalf("Normalize() diagnostics = %s, want dynamic-byte-array bound error", diagnosticSummary(result.Diagnostics))
	}
}

func TestNormalize_rejectsEncodedSizeOverflow(t *testing.T) {
	set := mutableFixtureDescriptorSet(t)
	field := fixtureField(t, fixtureMessage(t, set, "PhaseTwoOneV1ServiceSetRequest"), "value")
	field.Label = descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum()
	options := fieldOptions(t, field)
	options.Array = &optionsv1alpha1.ArrayLayout{
		LengthFieldWidth: optionsv1alpha1.LengthFieldWidth_LENGTH_FIELD_WIDTH_NONE,
		FixedItems:       math.MaxUint64,
	}
	proto.SetExtension(field.GetOptions(), optionsv1alpha1.E_Field, options)

	result := normalizeMutableFixture(t, set)
	if !hasDiagnosticMessage(result.Diagnostics, "SIP-ARRAY-001", "supported uint64 bound") {
		t.Fatalf("Normalize() diagnostics = %s, want encoded-size overflow error", diagnosticSummary(result.Diagnostics))
	}
}

func TestNormalize_rejectsNestedAggregateEncodedSizeOverflow(t *testing.T) {
	set := mutableFixtureDescriptorSet(t)
	child := fixtureMessage(t, set, "PhaseTwoOneV1ServiceSetResponse")
	childField := fixtureField(t, child, "accepted_value")
	childField.Label = descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum()
	childOptions := fieldOptions(t, childField)
	childOptions.Array = &optionsv1alpha1.ArrayLayout{
		LengthFieldWidth: optionsv1alpha1.LengthFieldWidth_LENGTH_FIELD_WIDTH_NONE,
		FixedItems:       math.MaxUint64,
	}
	proto.SetExtension(childField.GetOptions(), optionsv1alpha1.E_Field, childOptions)

	parent := fixtureMessage(t, set, "PhaseTwoOneV1ServiceSetRequest")
	nestedName := "response"
	nestedNumber := int32(2)
	nestedLabel := descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL
	nestedKind := descriptorpb.FieldDescriptorProto_TYPE_MESSAGE
	nestedType := ".io.github.someip_proto.test.v1alpha1.PhaseTwoOneV1ServiceSetResponse"
	nested := &descriptorpb.FieldDescriptorProto{
		Name:     &nestedName,
		Number:   &nestedNumber,
		Label:    &nestedLabel,
		Type:     &nestedKind,
		TypeName: &nestedType,
		Options:  &descriptorpb.FieldOptions{},
	}
	proto.SetExtension(nested.Options, optionsv1alpha1.E_Field, &optionsv1alpha1.FieldOptionsSpec{
		WireOrder:       2,
		MaxEncodedBytes: math.MaxUint64,
	})
	parent.Field = append(parent.Field, nested)

	result := normalizeMutableFixture(t, set)
	if !hasDiagnosticMessage(result.Diagnostics, "SIP-LAYOUT-002", "encoded-size calculation exceeds") {
		t.Fatalf("Normalize() diagnostics = %s, want nested aggregate encoded-size overflow", diagnosticSummary(result.Diagnostics))
	}
}

func TestNormalize_rejectsPaddingEncodedSizeOverflow(t *testing.T) {
	set := mutableFixtureDescriptorSet(t)
	message := fixtureMessage(t, set, "PhaseTwoOneV1ServiceSetRequest")
	options := messageOptions(t, message)
	options.GetStructLayout().Padding = []*optionsv1alpha1.Padding{{
		Position:  &optionsv1alpha1.Padding_AtEnd{AtEnd: true},
		ByteCount: math.MaxUint64,
	}}
	proto.SetExtension(message.GetOptions(), optionsv1alpha1.E_Message, options)

	result := normalizeMutableFixture(t, set)
	if !hasDiagnosticMessage(result.Diagnostics, "SIP-LAYOUT-002", "encoded-size calculation exceeds") {
		t.Fatalf("Normalize() diagnostics = %s, want padding encoded-size overflow", diagnosticSummary(result.Diagnostics))
	}
}

func TestNormalize_rejectsUnionHeaderAndPaddingOverflow(t *testing.T) {
	set := mutableFixtureDescriptorSet(t)
	message := fixtureMessage(t, set, "UnequalFixedZeroWidthUnion")
	options := oneofOptions(t, message.GetOneofDecl()[0])
	options.PaddingBytes = math.MaxUint64
	options.MaxEncodedBytes = math.MaxUint64
	proto.SetExtension(message.GetOneofDecl()[0].GetOptions(), optionsv1alpha1.E_Oneof, options)

	result := normalizeMutableFixture(t, set)
	if !hasDiagnosticMessage(result.Diagnostics, "SIP-UNION-001", "encoded-size calculation exceeds") {
		t.Fatalf("Normalize() diagnostics = %s, want union header/padding encoded-size overflow", diagnosticSummary(result.Diagnostics))
	}
}

func TestSomeipMessageSize_rejectsEnvelopeOverflow(t *testing.T) {
	normalizer := &normalizer{}
	if got := normalizer.someipMessageSize(math.MaxUint64, descriptor.Location{}, ".example.Service"); got != math.MaxUint64 {
		t.Fatalf("someipMessageSize() = %d, want saturated maximum", got)
	}
	if !hasDiagnosticMessage(normalizer.diagnostics.diagnostics, "SIP-PROFILE-001", "maximum SOME/IP message size exceeds") {
		t.Fatalf("someipMessageSize() diagnostics = %s, want overflow error", diagnosticSummary(normalizer.diagnostics.diagnostics))
	}
}

func TestNormalize_rejectsPayloadAnalysisDepthOverflow(t *testing.T) {
	set := mutableFixtureDescriptorSet(t)
	file := fixtureFile(t, set, "io/github/someip_proto/test/v1alpha1/phase_2_1_schema_fixture.proto")
	const packageName = "io.github.someip_proto.test.v1alpha1"
	for index := 0; index < 5; index++ {
		name := fmt.Sprintf("AnalysisDepth%d", index)
		fieldName := "value"
		fieldNumber := int32(1)
		fieldLabel := descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL
		field := &descriptorpb.FieldDescriptorProto{
			Name:    &fieldName,
			Number:  &fieldNumber,
			Label:   &fieldLabel,
			Options: &descriptorpb.FieldOptions{},
		}
		fieldOptions := &optionsv1alpha1.FieldOptionsSpec{WireOrder: 1}
		if index == 4 {
			fieldKind := descriptorpb.FieldDescriptorProto_TYPE_UINT32
			field.Type = &fieldKind
			fieldOptions.ScalarRepresentation = optionsv1alpha1.ScalarRepresentation_SCALAR_REPRESENTATION_UNSIGNED_INTEGER_8
		} else {
			fieldKind := descriptorpb.FieldDescriptorProto_TYPE_MESSAGE
			nextType := "." + packageName + fmt.Sprintf(".AnalysisDepth%d", index+1)
			field.Type = &fieldKind
			field.TypeName = &nextType
		}
		proto.SetExtension(field.Options, optionsv1alpha1.E_Field, fieldOptions)
		message := &descriptorpb.DescriptorProto{
			Name:    &name,
			Field:   []*descriptorpb.FieldDescriptorProto{field},
			Options: &descriptorpb.MessageOptions{},
		}
		proto.SetExtension(message.Options, optionsv1alpha1.E_Message, &optionsv1alpha1.MessageOptionsSpec{
			StructLayout: &optionsv1alpha1.StructLayout{LengthFieldWidth: optionsv1alpha1.LengthFieldWidth_LENGTH_FIELD_WIDTH_NONE},
		})
		file.MessageType = append(file.MessageType, message)
	}
	request := fixtureMessage(t, set, "PhaseTwoOneV1ServiceSetRequest")
	field := fixtureField(t, request, "value")
	messageKind := descriptorpb.FieldDescriptorProto_TYPE_MESSAGE
	rootType := "." + packageName + ".AnalysisDepth0"
	field.Type = &messageKind
	field.TypeName = &rootType
	proto.SetExtension(field.Options, optionsv1alpha1.E_Field, &optionsv1alpha1.FieldOptionsSpec{WireOrder: 1})

	result := normalizeMutableFixtureWithLimits(t, set, "phase-2-1-valid", descriptor.Limits{MaxDescriptorDepth: 3})
	if !hasDiagnosticMessage(result.Diagnostics, "SIP-POLICY-003", "analysis limit") {
		t.Fatalf("Normalize() diagnostics = %s, want semantic payload-analysis depth error", diagnosticSummary(result.Diagnostics))
	}
}

func TestNormalize_acceptsContiguousUnionIDsIndependentOfDeclarationOrder(t *testing.T) {
	set := mutableFixtureDescriptorSet(t)
	message := fixtureMessage(t, set, "UnequalFixedZeroWidthUnion")
	first := fixtureField(t, message, "one_byte")
	second := fixtureField(t, message, "two_bytes")
	firstOptions := fieldOptions(t, first)
	secondOptions := fieldOptions(t, second)
	firstOptions.UnionTypeId = 2
	secondOptions.UnionTypeId = 1
	proto.SetExtension(first.GetOptions(), optionsv1alpha1.E_Field, firstOptions)
	proto.SetExtension(second.GetOptions(), optionsv1alpha1.E_Field, secondOptions)

	result := normalizeMutableFixture(t, set)
	if result.HasErrors() {
		t.Fatalf("Normalize() diagnostics = %s, want declaration-order-independent union IDs", diagnosticSummary(result.Diagnostics))
	}
}

func TestNormalize_materializesUnionDefaultsAndNullAlternative(t *testing.T) {
	set := mutableFixtureDescriptorSet(t)
	message := fixtureMessage(t, set, "UnequalFixedZeroWidthUnion")
	options := oneofOptions(t, message.GetOneofDecl()[0])
	options.DiscriminatorWidthBits = 0
	options.LengthFieldWidth = optionsv1alpha1.LengthFieldWidth_LENGTH_FIELD_WIDTH_UNSPECIFIED
	options.HeaderOrder = optionsv1alpha1.UnionHeaderOrder_UNION_HEADER_ORDER_UNSPECIFIED
	options.AllowsNull = true
	options.MaxEncodedBytes = 10
	proto.SetExtension(message.GetOneofDecl()[0].GetOptions(), optionsv1alpha1.E_Oneof, options)

	result := normalizeMutableFixture(t, set)
	if result.HasErrors() {
		t.Fatalf("Normalize() diagnostics = %s, want effective union defaults", diagnosticSummary(result.Diagnostics))
	}
	union := findUnion(t, result.Manifest, ".io.github.someip_proto.test.v1alpha1.UnequalFixedZeroWidthUnion")
	if got := union.GetOptions().GetDiscriminatorWidthBits(); got != 0 {
		t.Fatalf("declared discriminator width = %d, want unspecified", got)
	}
	if got := union.GetEffectiveLayout().GetDiscriminatorWidthBits(); got != 32 {
		t.Fatalf("effective discriminator width = %d, want 32", got)
	}
	if got := union.GetEffectiveLayout().GetLengthFieldWidth(); got != optionsv1alpha1.LengthFieldWidth_LENGTH_FIELD_WIDTH_32 {
		t.Fatalf("effective length width = %s, want LENGTH_FIELD_WIDTH_32", got)
	}
	if got := union.GetEffectiveLayout().GetHeaderOrder(); got != optionsv1alpha1.UnionHeaderOrder_UNION_HEADER_ORDER_LENGTH_THEN_TYPE {
		t.Fatalf("effective header order = %s, want LENGTH_THEN_TYPE", got)
	}
	if !union.GetEffectiveLayout().GetAllowsNull() || union.GetEffectiveLayout().GetNullTypeId() != 0 {
		t.Fatalf("effective NULL layout = %#v, want type ID zero enabled", union.GetEffectiveLayout())
	}
	if alternatives := union.GetAlternatives(); len(alternatives) != 3 || alternatives[0].GetTypeId() != 0 || alternatives[0].GetName() != "NULL" {
		t.Fatalf("union alternatives = %#v, want leading normalized NULL alternative", alternatives)
	}
	if got := union.GetEncodedSize().GetMinimumBytes(); got != 8 {
		t.Fatalf("union minimum bytes = %d, want 8 for the header-only NULL form", got)
	}
}

func TestNormalize_validatesUnboundAnnotatedServices(t *testing.T) {
	set := mutableFixtureDescriptorSet(t)
	file := fixtureFile(t, set, "io/github/someip_proto/test/v1alpha1/phase_2_1_schema_fixture.proto")
	serviceName := "UnboundInvalidService"
	methodName := "StreamWithoutMetadata"
	inputType := ".io.github.someip_proto.test.v1alpha1.PhaseTwoOneV1ServiceSetRequest"
	outputType := ".io.github.someip_proto.test.v1alpha1.PhaseTwoOneV1ServiceSetResponse"
	streaming := true
	service := &descriptorpb.ServiceDescriptorProto{
		Name:    &serviceName,
		Options: &descriptorpb.ServiceOptions{},
		Method: []*descriptorpb.MethodDescriptorProto{{
			Name:            &methodName,
			InputType:       &inputType,
			OutputType:      &outputType,
			ClientStreaming: &streaming,
		}},
	}
	proto.SetExtension(service.Options, optionsv1alpha1.E_Service, &optionsv1alpha1.ServiceOptionsSpec{
		InterfaceMajorVersion: 1,
		ServiceLineage:        &optionsv1alpha1.ServiceLineage{Id: "fixture.unbound"},
	})
	file.Service = append(file.Service, service)

	result := normalizeMutableFixture(t, set)
	if !hasDiagnostic(result.Diagnostics, "SIP-POLICY-002", manifestv1alpha1.DiagnosticSeverity_DIAGNOSTIC_SEVERITY_ERROR) {
		t.Fatalf("Normalize() diagnostics = %s, want unbound service policy error", diagnosticSummary(result.Diagnostics))
	}
}

func TestNormalize_allowsEventOnlyService(t *testing.T) {
	set := mutableFixtureDescriptorSet(t)
	service := fixtureService(t, set, "PhaseTwoOneV1Service")
	service.Method = nil
	options, ok := proto.GetExtension(service.GetOptions(), optionsv1alpha1.E_Service).(*optionsv1alpha1.ServiceOptionsSpec)
	if !ok || options == nil {
		t.Fatal("fixture service lacks project options")
	}
	options = proto.Clone(options).(*optionsv1alpha1.ServiceOptionsSpec)
	options.Events = []*optionsv1alpha1.Event{{
		Name:        "changed",
		Id:          1,
		PayloadType: &optionsv1alpha1.ProtobufReference{Fqn: ".io.github.someip_proto.test.v1alpha1.PhaseTwoOneV1ServiceSetResponse"},
		Publication: &optionsv1alpha1.PublicationPolicy{Kind: optionsv1alpha1.PublicationPolicyKind_PUBLICATION_POLICY_KIND_ON_CHANGE},
	}}
	proto.SetExtension(service.GetOptions(), optionsv1alpha1.E_Service, options)

	result := normalizeMutableFixture(t, set)
	if result.HasErrors() {
		t.Fatalf("Normalize() diagnostics = %s, want event-only service acceptance", diagnosticSummary(result.Diagnostics))
	}
	members := findService(t, result.Manifest, ".io.github.someip_proto.test.v1alpha1.PhaseTwoOneV1Service").GetMembers()
	if len(members) != 1 || members[0].GetKind() != "event" || members[0].GetName() != "changed" {
		t.Fatalf("event-only service members = %#v, want normalized event", members)
	}
}

func TestNormalize_rejectsUnapprovedImportedAnnotatedService(t *testing.T) {
	set := mutableFixtureDescriptorSet(t)
	profileFile := fixtureFile(t, set, "io/github/someip_proto/test/v1alpha1/phase_2_1_schema_fixture.proto")
	var importedService *descriptorpb.ServiceDescriptorProto
	retained := make([]*descriptorpb.ServiceDescriptorProto, 0, len(profileFile.GetService()))
	for _, service := range profileFile.GetService() {
		if service.GetName() == "PhaseTwoOneV1Service" {
			importedService = proto.Clone(service).(*descriptorpb.ServiceDescriptorProto)
			continue
		}
		retained = append(retained, service)
	}
	if importedService == nil {
		t.Fatal("fixture service to move into an external file was not found")
	}
	profileFile.Service = retained
	set.File = append(set.File, &descriptorpb.FileDescriptorProto{
		Name:       proto.String("example/dependency_service.proto"),
		Package:    proto.String(profileFile.GetPackage()),
		Syntax:     proto.String("proto3"),
		Dependency: []string{profileFile.GetName(), "io/github/someip_proto/options/v1alpha1/someip_options.proto"},
		Service:    []*descriptorpb.ServiceDescriptorProto{importedService},
	})

	result := normalizeMutableFixture(t, set)
	if !hasDiagnosticMessage(result.Diagnostics, "SIP-ARTIFACT-001", "annotated service") {
		t.Fatalf("Normalize() diagnostics = %s, want unapproved external service provenance error", diagnosticSummary(result.Diagnostics))
	}
}

func TestNormalize_rejectsUnapprovedImportedPayload(t *testing.T) {
	set := mutableFixtureDescriptorSet(t)
	profileFile := fixtureFile(t, set, "io/github/someip_proto/test/v1alpha1/phase_2_1_schema_fixture.proto")
	var importedPayload *descriptorpb.DescriptorProto
	retained := make([]*descriptorpb.DescriptorProto, 0, len(profileFile.GetMessageType()))
	for _, message := range profileFile.GetMessageType() {
		if message.GetName() == "PhaseTwoOneV1ServiceSetResponse" {
			importedPayload = proto.Clone(message).(*descriptorpb.DescriptorProto)
			continue
		}
		retained = append(retained, message)
	}
	if importedPayload == nil {
		t.Fatal("fixture payload to move into an external file was not found")
	}
	profileFile.MessageType = retained
	profileFile.Dependency = append(profileFile.Dependency, "example/dependency_payload.proto")
	externalFile := &descriptorpb.FileDescriptorProto{
		Name:        proto.String("example/dependency_payload.proto"),
		Package:     proto.String(profileFile.GetPackage()),
		Syntax:      proto.String("proto3"),
		Dependency:  []string{"io/github/someip_proto/options/v1alpha1/someip_options.proto"},
		MessageType: []*descriptorpb.DescriptorProto{importedPayload},
		Options:     &descriptorpb.FileOptions{},
	}
	proto.SetExtension(externalFile.Options, optionsv1alpha1.E_File, &optionsv1alpha1.FileOptionsSpec{
		DefaultByteOrder: optionsv1alpha1.ByteOrder_BYTE_ORDER_BIG_ENDIAN,
	})
	set.File = append(set.File, externalFile)

	result := normalizeMutableFixture(t, set)
	if !hasDiagnosticMessage(result.Diagnostics, "SIP-ARTIFACT-001", "serializable payload") {
		t.Fatalf("Normalize() diagnostics = %s, want unapproved external payload provenance error", diagnosticSummary(result.Diagnostics))
	}
}

func TestNormalize_materializesLogicalFieldContracts(t *testing.T) {
	set := mutableFixtureDescriptorSet(t)
	service := fixtureService(t, set, "PhaseTwoOneV1Service")
	options, ok := proto.GetExtension(service.GetOptions(), optionsv1alpha1.E_Service).(*optionsv1alpha1.ServiceOptionsSpec)
	if !ok || options == nil {
		t.Fatal("fixture service lacks project options")
	}
	options = proto.Clone(options).(*optionsv1alpha1.ServiceOptionsSpec)
	options.Fields = append(options.Fields, &optionsv1alpha1.ServiceField{
		Name:        "state",
		ValueType:   &optionsv1alpha1.ProtobufReference{Fqn: ".io.github.someip_proto.test.v1alpha1.PhaseTwoOneV1ServiceSetResponse"},
		NotifierId:  0x8001,
		Publication: &optionsv1alpha1.PublicationPolicy{Kind: optionsv1alpha1.PublicationPolicyKind_PUBLICATION_POLICY_KIND_ON_CHANGE},
	})
	proto.SetExtension(service.GetOptions(), optionsv1alpha1.E_Service, options)
	profile := deploymentProfileOptions(t, fixtureFile(t, set, "io/github/someip_proto/test/v1alpha1/phase_2_1_schema_fixture.proto"))
	profile.GetProviderInstances()[0].GetServiceVersionBindings()[0].MemberTpConfigurations = append(
		profile.GetProviderInstances()[0].GetServiceVersionBindings()[0].GetMemberTpConfigurations(),
		&optionsv1alpha1.MemberTpConfiguration{MemberId: 0x8001, TpWhenNeeded: false},
	)

	result := normalizeMutableFixture(t, set)
	if result.HasErrors() {
		t.Fatalf("Normalize() diagnostics = %s, want logical field manifest", diagnosticSummary(result.Diagnostics))
	}
	fields := findService(t, result.Manifest, ".io.github.someip_proto.test.v1alpha1.PhaseTwoOneV1Service").GetFields()
	if len(fields) != 1 || fields[0].GetField().GetNotifierId() != 0x8001 || fields[0].GetField().GetValueType().GetFqn() == "" {
		t.Fatalf("logical field manifest = %#v, want normalized notifier contract", fields)
	}
}

func TestNormalize_rejectsEmptyExceptionPayload(t *testing.T) {
	set := mutableFixtureDescriptorSet(t)
	service := fixtureService(t, set, "PhaseTwoOneV1Service")
	method := service.GetMethod()[0]
	options, ok := proto.GetExtension(method.GetOptions(), optionsv1alpha1.E_Method).(*optionsv1alpha1.MethodOptionsSpec)
	if !ok || options == nil {
		t.Fatal("fixture method lacks project options")
	}
	options = proto.Clone(options).(*optionsv1alpha1.MethodOptionsSpec)
	options.ResponsePolicy = optionsv1alpha1.ResponsePolicy_RESPONSE_POLICY_RESPONSE_OR_EXCEPTION
	options.ExceptionPayloadType = &optionsv1alpha1.ProtobufReference{Fqn: ".google.protobuf.Empty"}
	proto.SetExtension(method.GetOptions(), optionsv1alpha1.E_Method, options)
	set.File = append(set.File, &descriptorpb.FileDescriptorProto{
		Name:           proto.String(string(emptypb.File_google_protobuf_empty_proto.Path())),
		Package:        proto.String(string(emptypb.File_google_protobuf_empty_proto.Package())),
		MessageType:    protodesc.ToFileDescriptorProto(emptypb.File_google_protobuf_empty_proto).GetMessageType(),
		SourceCodeInfo: &descriptorpb.SourceCodeInfo{Location: []*descriptorpb.SourceCodeInfo_Location{{Span: []int32{0, 0, 0}}}},
		Syntax:         proto.String("proto3"),
	})

	result := normalizeMutableFixture(t, set)
	if !hasDiagnosticMessage(result.Diagnostics, "SIP-MEMBER-001", "cannot use google.protobuf.Empty as an exception payload") {
		t.Fatalf("Normalize() diagnostics = %s, want Empty exception-payload error", diagnosticSummary(result.Diagnostics))
	}
}

func TestNormalize_allowsDomainWithoutScale(t *testing.T) {
	set := mutableFixtureDescriptorSet(t)
	message := fixtureMessage(t, set, "PhaseTwoOneV1ServiceSetRequest")
	field := fixtureField(t, message, "value")
	fieldOptions := fieldOptions(t, field)
	fieldOptions.Domain = &optionsv1alpha1.ValueDomain{
		Minimum: &optionsv1alpha1.Rational{Numerator: 0, Denominator: 1},
		Maximum: &optionsv1alpha1.Rational{Numerator: 100, Denominator: 1},
	}
	proto.SetExtension(field.GetOptions(), optionsv1alpha1.E_Field, fieldOptions)

	result := normalizeMutableFixture(t, set)
	if result.HasErrors() {
		t.Fatalf("Normalize() diagnostics = %s, want optional value-domain scale", diagnosticSummary(result.Diagnostics))
	}
}

func TestNormalize_requiresEveryBitForClosedBitfield(t *testing.T) {
	set := mutableFixtureDescriptorSet(t)
	field := fixtureField(t, fixtureMessage(t, set, "PhaseTwoOneV1ServiceSetRequest"), "value")
	options := fieldOptions(t, field)
	options.ScalarRepresentation = optionsv1alpha1.ScalarRepresentation_SCALAR_REPRESENTATION_UNSIGNED_INTEGER_8
	options.Bitfield = &optionsv1alpha1.BitfieldLayout{
		WidthBits: 8,
		Closed:    true,
		NamedBits: []*optionsv1alpha1.NamedBit{{BitIndex: 0, Name: "enabled"}},
	}
	proto.SetExtension(field.GetOptions(), optionsv1alpha1.E_Field, options)

	result := normalizeMutableFixture(t, set)
	if !hasDiagnosticMessage(result.Diagnostics, "SIP-ENUM-001", "must name every bit") {
		t.Fatalf("Normalize() diagnostics = %s, want closed-bitfield coverage error", diagnosticSummary(result.Diagnostics))
	}
}

func TestNormalize_rejectsOptionalUnionAlternative(t *testing.T) {
	set := mutableFixtureDescriptorSet(t)
	field := fixtureField(t, fixtureMessage(t, set, "UnequalFixedZeroWidthUnion"), "one_byte")
	options := fieldOptions(t, field)
	options.OptionalElement = true
	proto.SetExtension(field.GetOptions(), optionsv1alpha1.E_Field, options)

	result := normalizeMutableFixture(t, set)
	if !hasDiagnosticMessage(result.Diagnostics, "SIP-UNION-001", "cannot use optional-element framing") {
		t.Fatalf("Normalize() diagnostics = %s, want union optional-element error", diagnosticSummary(result.Diagnostics))
	}
}

func TestNormalize_requiresMaximumForFixedDynamicCollection(t *testing.T) {
	set := mutableFixtureDescriptorSet(t)
	field := fixtureField(t, fixtureMessage(t, set, "PhaseTwoOneV1ServiceSetRequest"), "value")
	field.Type = descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum()
	field.Label = descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum()
	field.TypeName = nil
	options := fieldOptions(t, field)
	options.ScalarRepresentation = optionsv1alpha1.ScalarRepresentation_SCALAR_REPRESENTATION_UNSPECIFIED
	options.String_ = &optionsv1alpha1.StringLayout{
		Encoding:         optionsv1alpha1.StringEncoding_STRING_ENCODING_UTF8,
		LengthFieldWidth: optionsv1alpha1.LengthFieldWidth_LENGTH_FIELD_WIDTH_8,
		MaxEncodedBytes:  8,
	}
	options.Array = &optionsv1alpha1.ArrayLayout{
		LengthFieldWidth: optionsv1alpha1.LengthFieldWidth_LENGTH_FIELD_WIDTH_NONE,
		FixedItems:       2,
	}
	options.MaxEncodedBytes = 0
	proto.SetExtension(field.GetOptions(), optionsv1alpha1.E_Field, options)

	result := normalizeMutableFixture(t, set)
	if !hasDiagnosticMessage(result.Diagnostics, "SIP-ARRAY-001", "variable-element array field") {
		t.Fatalf("Normalize() diagnostics = %s, want fixed dynamic-array maximum error", diagnosticSummary(result.Diagnostics))
	}
}

func TestNormalize_analyzesUnionAlternativeAlignment(t *testing.T) {
	set := mutableFixtureDescriptorSet(t)
	field := fixtureField(t, fixtureMessage(t, set, "UnequalFixedZeroWidthUnion"), "one_byte")
	options := fieldOptions(t, field)
	options.AlignmentBytes = 4
	proto.SetExtension(field.GetOptions(), optionsv1alpha1.E_Field, options)

	result := normalizeMutableFixture(t, set)
	if !hasDiagnostic(result.Diagnostics, "SIP-LAYOUT-003", manifestv1alpha1.DiagnosticSeverity_DIAGNOSTIC_SEVERITY_WARNING) {
		t.Fatalf("Normalize() diagnostics = %s, want union-alternative alignment warning", diagnosticSummary(result.Diagnostics))
	}
	union := findUnion(t, result.Manifest, ".io.github.someip_proto.test.v1alpha1.UnequalFixedZeroWidthUnion")
	alternative := union.GetAlternatives()[0]
	if alignment := alternative.GetAlignment(); alignment.GetProven() || alignment.GetMinimumOffsetBytes() != 17 || alignment.GetMaximumOffsetBytes() != 17 {
		t.Fatalf("union alternative alignment = %#v, want unproven offset 17", alignment)
	}
}

func TestNormalize_analyzesNestedPayloadAlignment(t *testing.T) {
	set := mutableFixtureDescriptorSet(t)
	parent := fixtureMessage(t, set, "PhaseTwoOneV1ServiceSetRequest")
	leading := fixtureField(t, parent, "value")
	leadingOptions := fieldOptions(t, leading)
	leadingOptions.ScalarRepresentation = optionsv1alpha1.ScalarRepresentation_SCALAR_REPRESENTATION_UNSIGNED_INTEGER_8
	proto.SetExtension(leading.GetOptions(), optionsv1alpha1.E_Field, leadingOptions)

	nestedName := "nested_response"
	nestedType := ".io.github.someip_proto.test.v1alpha1.PhaseTwoOneV1ServiceSetResponse"
	nestedNumber := int32(2)
	nestedLabel := descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL
	nestedKind := descriptorpb.FieldDescriptorProto_TYPE_MESSAGE
	nested := &descriptorpb.FieldDescriptorProto{
		Name:     &nestedName,
		Number:   &nestedNumber,
		Label:    &nestedLabel,
		Type:     &nestedKind,
		TypeName: &nestedType,
		Options:  &descriptorpb.FieldOptions{},
	}
	proto.SetExtension(nested.GetOptions(), optionsv1alpha1.E_Field, &optionsv1alpha1.FieldOptionsSpec{WireOrder: 2})
	parent.Field = append(parent.Field, nested)

	child := fixtureMessage(t, set, "PhaseTwoOneV1ServiceSetResponse")
	childField := fixtureField(t, child, "accepted_value")
	childOptions := fieldOptions(t, childField)
	childOptions.AlignmentBytes = 4
	proto.SetExtension(childField.GetOptions(), optionsv1alpha1.E_Field, childOptions)

	result := normalizeMutableFixture(t, set)
	if !hasDiagnostic(result.Diagnostics, "SIP-LAYOUT-003", manifestv1alpha1.DiagnosticSeverity_DIAGNOSTIC_SEVERITY_WARNING) {
		t.Fatalf("Normalize() diagnostics = %s, want nested alignment warning", diagnosticSummary(result.Diagnostics))
	}
	payload := findPayload(t, result.Manifest, ".io.github.someip_proto.test.v1alpha1.PhaseTwoOneV1ServiceSetResponse")
	member := payload.GetLayoutElements()[0].GetMember()
	if alignment := member.GetAlignment(); alignment.GetProven() || alignment.GetMinimumOffsetBytes() != 16 || alignment.GetMaximumOffsetBytes() != 17 {
		t.Fatalf("nested member alignment = %#v, want aggregate offset range 16..17", alignment)
	}
}

func TestMaximumUDPApplicationPayload_respectsMemberEndpointOverrides(t *testing.T) {
	service := &serviceInfo{memberPayloadBounds: map[uint32][]memberDirectionalBound{
		1: {{direction: manifestv1alpha1.MemberDirection_MEMBER_DIRECTION_REQUEST, size: fixedSize(100)}},
		2: {{direction: manifestv1alpha1.MemberDirection_MEMBER_DIRECTION_REQUEST, size: fixedSize(500)}},
	}}
	provider := &optionsv1alpha1.ProviderInstance{
		DefaultEndpointName: "tcp",
		Endpoints: []*optionsv1alpha1.Endpoint{
			{Name: "tcp", Transport: optionsv1alpha1.TransportProtocol_TRANSPORT_PROTOCOL_TCP},
			{Name: "udp", Transport: optionsv1alpha1.TransportProtocol_TRANSPORT_PROTOCOL_UDP},
		},
	}
	binding := &optionsv1alpha1.ServiceVersionBinding{
		MemberEndpointOverrides: []*optionsv1alpha1.MemberEndpointOverride{{MemberId: 1, EndpointName: "udp"}},
	}
	if got, usesUDP := maximumUDPApplicationPayload(service, binding, provider); !usesUDP || got != 100 {
		t.Fatalf("TCP default with small UDP override = (%d, %t), want (100, true)", got, usesUDP)
	}

	provider.DefaultEndpointName = "udp"
	binding.MemberEndpointOverrides = []*optionsv1alpha1.MemberEndpointOverride{{MemberId: 2, EndpointName: "tcp"}}
	if got, usesUDP := maximumUDPApplicationPayload(service, binding, provider); !usesUDP || got != 100 {
		t.Fatalf("UDP default with large TCP override = (%d, %t), want (100, true)", got, usesUDP)
	}
}

func loadFixtureSet(t *testing.T) *descriptor.Set {
	t.Helper()
	input := fixtureDescriptorSet(t, testv1alpha1.File_io_github_someip_proto_test_v1alpha1_phase_2_1_schema_fixture_proto)
	set, err := descriptor.Load(input, descriptor.DefaultLimits())
	if err != nil {
		t.Fatalf("descriptor.Load() error = %v", err)
	}
	return set
}

func mutableFixtureDescriptorSet(t *testing.T) *descriptorpb.FileDescriptorSet {
	t.Helper()
	input := fixtureDescriptorSet(t, testv1alpha1.File_io_github_someip_proto_test_v1alpha1_phase_2_1_schema_fixture_proto)
	set := &descriptorpb.FileDescriptorSet{}
	if err := proto.Unmarshal(input, set); err != nil {
		t.Fatalf("unmarshal fixture FileDescriptorSet: %v", err)
	}
	return set
}

func normalizeMutableFixture(t *testing.T, set *descriptorpb.FileDescriptorSet) *Result {
	return normalizeMutableFixtureForProfile(t, set, "phase-2-1-valid")
}

func normalizeMutableFixtureForProfile(t *testing.T, set *descriptorpb.FileDescriptorSet, profile string) *Result {
	return normalizeMutableFixtureWithLimits(t, set, profile, descriptor.DefaultLimits())
}

func normalizeMutableFixtureWithLimits(t *testing.T, set *descriptorpb.FileDescriptorSet, profile string, limits descriptor.Limits) *Result {
	t.Helper()
	for _, file := range set.GetFile() {
		testutil.CompleteSourceCodeInfo(file)
	}
	input, err := proto.MarshalOptions{Deterministic: true}.Marshal(set)
	if err != nil {
		t.Fatalf("marshal mutated fixture FileDescriptorSet: %v", err)
	}
	loaded, err := descriptor.Load(input, limits)
	if err != nil {
		t.Fatalf("descriptor.Load() mutated fixture error: %v", err)
	}
	return Normalize(loaded, profile, DefaultConfig())
}

func fixtureMessage(t *testing.T, set *descriptorpb.FileDescriptorSet, name string) *descriptorpb.DescriptorProto {
	t.Helper()
	for _, file := range set.GetFile() {
		for _, message := range file.GetMessageType() {
			if message.GetName() == name {
				return message
			}
		}
	}
	t.Fatalf("fixture message %q was not found", name)
	return nil
}

func fixtureFile(t *testing.T, set *descriptorpb.FileDescriptorSet, name string) *descriptorpb.FileDescriptorProto {
	t.Helper()
	for _, file := range set.GetFile() {
		if file.GetName() == name {
			return file
		}
	}
	t.Fatalf("fixture file %q was not found", name)
	return nil
}

func fixtureService(t *testing.T, set *descriptorpb.FileDescriptorSet, name string) *descriptorpb.ServiceDescriptorProto {
	t.Helper()
	for _, file := range set.GetFile() {
		for _, service := range file.GetService() {
			if service.GetName() == name {
				return service
			}
		}
	}
	t.Fatalf("fixture service %q was not found", name)
	return nil
}

func fixtureField(t *testing.T, message *descriptorpb.DescriptorProto, name string) *descriptorpb.FieldDescriptorProto {
	t.Helper()
	for _, field := range message.GetField() {
		if field.GetName() == name {
			return field
		}
	}
	t.Fatalf("fixture field %q was not found", name)
	return nil
}

func messageOptions(t *testing.T, message *descriptorpb.DescriptorProto) *optionsv1alpha1.MessageOptionsSpec {
	t.Helper()
	options, ok := proto.GetExtension(message.GetOptions(), optionsv1alpha1.E_Message).(*optionsv1alpha1.MessageOptionsSpec)
	if !ok || options == nil {
		t.Fatalf("fixture message %q lacks project options", message.GetName())
	}
	return proto.Clone(options).(*optionsv1alpha1.MessageOptionsSpec)
}

func fieldOptions(t *testing.T, field *descriptorpb.FieldDescriptorProto) *optionsv1alpha1.FieldOptionsSpec {
	t.Helper()
	options, ok := proto.GetExtension(field.GetOptions(), optionsv1alpha1.E_Field).(*optionsv1alpha1.FieldOptionsSpec)
	if !ok || options == nil {
		t.Fatalf("fixture field %q lacks project options", field.GetName())
	}
	return proto.Clone(options).(*optionsv1alpha1.FieldOptionsSpec)
}

func oneofOptions(t *testing.T, oneof *descriptorpb.OneofDescriptorProto) *optionsv1alpha1.OneofOptionsSpec {
	t.Helper()
	options, ok := proto.GetExtension(oneof.GetOptions(), optionsv1alpha1.E_Oneof).(*optionsv1alpha1.OneofOptionsSpec)
	if !ok || options == nil {
		t.Fatalf("fixture oneof %q lacks project options", oneof.GetName())
	}
	return proto.Clone(options).(*optionsv1alpha1.OneofOptionsSpec)
}

func deploymentProfileOptions(t *testing.T, file *descriptorpb.FileDescriptorProto) *optionsv1alpha1.DeploymentProfile {
	t.Helper()
	options, ok := proto.GetExtension(file.GetOptions(), optionsv1alpha1.E_File).(*optionsv1alpha1.FileOptionsSpec)
	if !ok || options == nil || options.GetDeploymentProfile() == nil {
		t.Fatalf("fixture file %q lacks deployment profile options", file.GetName())
	}
	cloned := proto.Clone(options).(*optionsv1alpha1.FileOptionsSpec)
	proto.SetExtension(file.GetOptions(), optionsv1alpha1.E_File, cloned)
	return cloned.GetDeploymentProfile()
}

func fixtureDescriptorSet(t testing.TB, file protoreflect.FileDescriptor) []byte {
	t.Helper()
	byPath := make(map[string]*descriptorpb.FileDescriptorProto)
	var addFile func(protoreflect.FileDescriptor)
	addFile = func(descriptorFile protoreflect.FileDescriptor) {
		if _, found := byPath[string(descriptorFile.Path())]; found {
			return
		}
		byPath[string(descriptorFile.Path())] = protodesc.ToFileDescriptorProto(descriptorFile)
		imports := descriptorFile.Imports()
		for index := range imports.Len() {
			addFile(imports.Get(index).FileDescriptor)
		}
	}
	addFile(file)
	paths := make([]string, 0, len(byPath))
	for path := range byPath {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	set := &descriptorpb.FileDescriptorSet{File: make([]*descriptorpb.FileDescriptorProto, 0, len(paths))}
	for _, path := range paths {
		descriptorFile := byPath[path]
		testutil.CompleteSourceCodeInfo(descriptorFile)
		set.File = append(set.File, descriptorFile)
	}
	input, err := proto.MarshalOptions{Deterministic: true}.Marshal(set)
	if err != nil {
		t.Fatalf("proto.Marshal() error = %v", err)
	}
	return input
}

func FuzzNormalize(f *testing.F) {
	f.Add([]byte{})
	f.Add(fixtureDescriptorSet(f, testv1alpha1.File_io_github_someip_proto_test_v1alpha1_phase_2_1_schema_fixture_proto))
	limits := descriptor.Limits{
		MaxInputBytes:  1 << 20,
		MaxFiles:       1_000,
		MaxDescriptors: 10_000,
	}
	f.Fuzz(func(t *testing.T, input []byte) {
		if len(input) > limits.MaxInputBytes {
			t.Skip()
		}
		set, err := descriptor.Load(input, limits)
		if err != nil {
			return
		}
		for _, result := range ValidateAll(set, DefaultConfig()) {
			if result == nil || result.Manifest == nil {
				t.Fatal("ValidateAll() returned an incomplete result")
			}
		}
	})
}

func hasDiagnostic(diagnostics []*manifestv1alpha1.Diagnostic, rule string, severity manifestv1alpha1.DiagnosticSeverity) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.GetRuleCode() == rule && diagnostic.GetSeverity() == severity {
			return true
		}
	}
	return false
}

func hasDiagnosticMessage(diagnostics []*manifestv1alpha1.Diagnostic, rule, text string) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.GetRuleCode() == rule && strings.Contains(diagnostic.GetMessage(), text) {
			return true
		}
	}
	return false
}

func diagnosticSummary(diagnostics []*manifestv1alpha1.Diagnostic) string {
	entries := make([]string, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		entries = append(entries, diagnostic.GetRuleCode()+":"+diagnostic.GetMessage())
	}
	return "[" + strings.Join(entries, "; ") + "]"
}

func findUnion(t *testing.T, manifest *manifestv1alpha1.Manifest, name string) *manifestv1alpha1.UnionManifest {
	t.Helper()
	for _, payload := range manifest.GetPayloadTypes() {
		if payload.GetType().GetFqn() != name {
			continue
		}
		for _, element := range payload.GetLayoutElements() {
			if union := element.GetUnion(); union != nil {
				return union
			}
		}
	}
	t.Fatalf("union payload %s was not found", name)
	return nil
}

func findService(t *testing.T, manifest *manifestv1alpha1.Manifest, name string) *manifestv1alpha1.ServiceManifest {
	t.Helper()
	for _, service := range manifest.GetServices() {
		if service.GetService().GetFqn() == name {
			return service
		}
	}
	t.Fatalf("service %s was not found", name)
	return nil
}

func findPayload(t *testing.T, manifest *manifestv1alpha1.Manifest, name string) *manifestv1alpha1.PayloadTypeManifest {
	t.Helper()
	for _, payload := range manifest.GetPayloadTypes() {
		if payload.GetType().GetFqn() == name {
			return payload
		}
	}
	t.Fatalf("payload %s was not found", name)
	return nil
}

func findEnum(t *testing.T, manifest *manifestv1alpha1.Manifest, name string) *manifestv1alpha1.EnumManifest {
	t.Helper()
	for _, enum := range manifest.GetEnumTypes() {
		if enum.GetType().GetFqn() == name {
			return enum
		}
	}
	t.Fatalf("enum %s was not found", name)
	return nil
}
