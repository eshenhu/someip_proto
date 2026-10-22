package compatibility

import (
	"sort"
	"testing"

	optionsv1alpha1 "github.com/eshenhu/someip_proto/gen/go/io/github/someip_proto/options/v1alpha1"
	testv1alpha1 "github.com/eshenhu/someip_proto/gen/go/io/github/someip_proto/test/v1alpha1"
	"github.com/eshenhu/someip_proto/internal/descriptor"
	"github.com/eshenhu/someip_proto/internal/semantic"
	"github.com/eshenhu/someip_proto/internal/testutil"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

func TestCompare_detectsMethodBehaviorChange(t *testing.T) {
	baseline := loadComparisonFixture(t, comparisonFixtureSet(t))
	currentSet := comparisonFixtureSet(t)
	method := comparisonMethod(t, currentSet, "PhaseTwoOneV1Service", "Set")
	methodOptions, ok := proto.GetExtension(method.GetOptions(), optionsv1alpha1.E_Method).(*optionsv1alpha1.MethodOptionsSpec)
	if !ok || methodOptions == nil {
		t.Fatal("fixture method lacks project options")
	}
	mutatedOptions := proto.Clone(methodOptions).(*optionsv1alpha1.MethodOptionsSpec)
	mutatedOptions.ReturnCodes = []*optionsv1alpha1.ReturnCode{{Code: 1, Name: "E_NOT_READY"}}
	proto.SetExtension(method.GetOptions(), optionsv1alpha1.E_Method, mutatedOptions)
	current := loadComparisonFixture(t, currentSet)

	result := Compare(baseline, current, "phase-2-1-valid", semantic.DefaultConfig())
	if !result.HasErrors() || !hasRule(result.Diagnostics, "SIP-COMPAT-001") {
		t.Fatalf("Compare() diagnostics = %#v, want method-behavior compatibility error", result.Diagnostics)
	}
}

func TestCompare_requiresProfilePatchForDeprecationMetadata(t *testing.T) {
	baseline := loadComparisonFixture(t, comparisonFixtureSet(t))
	currentSet := comparisonFixtureSet(t)
	field := comparisonField(t, currentSet, "PhaseTwoOneV1ServiceSetRequest", "value")
	field.GetOptions().Deprecated = proto.Bool(true)
	current := loadComparisonFixture(t, currentSet)

	result := Compare(baseline, current, "phase-2-1-valid", semantic.DefaultConfig())
	if !result.HasErrors() || !hasRule(result.Diagnostics, "SIP-COMPAT-001") {
		t.Fatalf("Compare() diagnostics = %#v, want profile-patch compatibility error", result.Diagnostics)
	}

	profileFile := comparisonFile(t, currentSet, "io/github/someip_proto/test/v1alpha1/phase_2_1_schema_fixture.proto")
	fileOptions, ok := proto.GetExtension(profileFile.GetOptions(), optionsv1alpha1.E_File).(*optionsv1alpha1.FileOptionsSpec)
	if !ok || fileOptions == nil {
		t.Fatal("fixture file lacks project options")
	}
	fileOptions = proto.Clone(fileOptions).(*optionsv1alpha1.FileOptionsSpec)
	fileOptions.GetDeploymentProfile().GetVersion().Patch = 1
	proto.SetExtension(profileFile.GetOptions(), optionsv1alpha1.E_File, fileOptions)
	current = loadComparisonFixture(t, currentSet)

	if result := Compare(baseline, current, "phase-2-1-valid", semantic.DefaultConfig()); result.HasErrors() {
		t.Fatalf("Compare() diagnostics = %#v, want accepted profile-patch transition", result.Diagnostics)
	}
}

func TestCompare_requiresProfilePatchForDocumentationMetadata(t *testing.T) {
	baseline := loadComparisonFixture(t, comparisonFixtureSet(t))
	currentSet := comparisonFixtureSet(t)
	profileFile := comparisonFile(t, currentSet, "io/github/someip_proto/test/v1alpha1/phase_2_1_schema_fixture.proto")
	commentPath, found := comparisonFieldPath(profileFile, "PhaseTwoOneV1ServiceSetRequest", "value")
	if !found {
		t.Fatal("fixture request value field was not found")
	}
	comment := "Updated contract documentation."
	commentLocation := comparisonSourceLocation(profileFile, commentPath)
	if commentLocation == nil {
		t.Fatal("fixture request value source location was not found")
	}
	commentLocation.LeadingComments = &comment
	current := loadComparisonFixture(t, currentSet)

	result := Compare(baseline, current, "phase-2-1-valid", semantic.DefaultConfig())
	if !result.HasErrors() || !hasRule(result.Diagnostics, "SIP-COMPAT-001") {
		t.Fatalf("Compare() diagnostics = %#v, want documentation patch-version error", result.Diagnostics)
	}

	fileOptions, ok := proto.GetExtension(profileFile.GetOptions(), optionsv1alpha1.E_File).(*optionsv1alpha1.FileOptionsSpec)
	if !ok || fileOptions == nil {
		t.Fatal("fixture file lacks project options")
	}
	fileOptions = proto.Clone(fileOptions).(*optionsv1alpha1.FileOptionsSpec)
	fileOptions.GetDeploymentProfile().GetVersion().Patch = 1
	proto.SetExtension(profileFile.GetOptions(), optionsv1alpha1.E_File, fileOptions)
	current = loadComparisonFixture(t, currentSet)

	if result := Compare(baseline, current, "phase-2-1-valid", semantic.DefaultConfig()); result.HasErrors() {
		t.Fatalf("Compare() diagnostics = %#v, want accepted documentation patch transition", result.Diagnostics)
	}
}

func TestCompare_requiresProfilePatchForFileDocumentationMetadata(t *testing.T) {
	baseline := loadComparisonFixture(t, comparisonFixtureSet(t))
	currentSet := comparisonFixtureSet(t)
	profileFile := comparisonFile(t, currentSet, "io/github/someip_proto/test/v1alpha1/phase_2_1_schema_fixture.proto")
	comment := "Updated file documentation."
	rootLocation := comparisonRootLocation(profileFile)
	rootLocation.LeadingComments = &comment
	current := loadComparisonFixture(t, currentSet)

	result := Compare(baseline, current, "phase-2-1-valid", semantic.DefaultConfig())
	if !result.HasErrors() || !hasRule(result.Diagnostics, "SIP-COMPAT-001") {
		t.Fatalf("Compare() diagnostics = %#v, want file documentation patch-version error", result.Diagnostics)
	}

	fileOptions, ok := proto.GetExtension(profileFile.GetOptions(), optionsv1alpha1.E_File).(*optionsv1alpha1.FileOptionsSpec)
	if !ok || fileOptions == nil {
		t.Fatal("fixture file lacks project options")
	}
	fileOptions = proto.Clone(fileOptions).(*optionsv1alpha1.FileOptionsSpec)
	fileOptions.GetDeploymentProfile().GetVersion().Patch = 1
	proto.SetExtension(profileFile.GetOptions(), optionsv1alpha1.E_File, fileOptions)
	current = loadComparisonFixture(t, currentSet)

	if result := Compare(baseline, current, "phase-2-1-valid", semantic.DefaultConfig()); result.HasErrors() {
		t.Fatalf("Compare() diagnostics = %#v, want accepted file documentation patch transition", result.Diagnostics)
	}
}

func TestCompare_requiresProfilePatchForDocumentationURI(t *testing.T) {
	baseline := loadComparisonFixture(t, comparisonFixtureSet(t))
	currentSet := comparisonFixtureSet(t)
	field := comparisonField(t, currentSet, "PhaseTwoOneV1ServiceSetRequest", "value")
	fieldOptions, ok := proto.GetExtension(field.GetOptions(), optionsv1alpha1.E_Field).(*optionsv1alpha1.FieldOptionsSpec)
	if !ok || fieldOptions == nil {
		t.Fatal("fixture field lacks project options")
	}
	fieldOptions = proto.Clone(fieldOptions).(*optionsv1alpha1.FieldOptionsSpec)
	fieldOptions.DocumentationUris = []string{"https://example.test/contracts/value"}
	proto.SetExtension(field.GetOptions(), optionsv1alpha1.E_Field, fieldOptions)
	current := loadComparisonFixture(t, currentSet)

	result := Compare(baseline, current, "phase-2-1-valid", semantic.DefaultConfig())
	if !result.HasErrors() || !hasRule(result.Diagnostics, "SIP-COMPAT-001") {
		t.Fatalf("Compare() diagnostics = %#v, want documentation URI patch-version error", result.Diagnostics)
	}

	profileFile := comparisonFile(t, currentSet, "io/github/someip_proto/test/v1alpha1/phase_2_1_schema_fixture.proto")
	fileOptions, ok := proto.GetExtension(profileFile.GetOptions(), optionsv1alpha1.E_File).(*optionsv1alpha1.FileOptionsSpec)
	if !ok || fileOptions == nil {
		t.Fatal("fixture file lacks project options")
	}
	fileOptions = proto.Clone(fileOptions).(*optionsv1alpha1.FileOptionsSpec)
	fileOptions.GetDeploymentProfile().GetVersion().Patch = 1
	proto.SetExtension(profileFile.GetOptions(), optionsv1alpha1.E_File, fileOptions)
	current = loadComparisonFixture(t, currentSet)

	if result := Compare(baseline, current, "phase-2-1-valid", semantic.DefaultConfig()); result.HasErrors() {
		t.Fatalf("Compare() diagnostics = %#v, want accepted documentation URI patch transition", result.Diagnostics)
	}
}

func TestCompare_rejectsIncompleteStableSourceMetadata(t *testing.T) {
	baseline := loadComparisonFixture(t, comparisonFixtureSet(t))
	current := loadComparisonFixture(t, comparisonFixtureSet(t))
	profileFile := comparisonFile(t, current.Raw, "io/github/someip_proto/test/v1alpha1/phase_2_1_schema_fixture.proto")
	path, found := comparisonFieldPath(profileFile, "PhaseTwoOneV1ServiceSetRequest", "value")
	if !found {
		t.Fatal("fixture request value field was not found")
	}
	// descriptor.Load rejects this state at the FDS boundary. Mutate the loaded
	// fixture to retain coverage for Compare's defensive metadata check when a
	// caller provides an in-memory descriptor set with incomplete metadata.
	removeSourceLocation(profileFile, path)
	current.InputSHA256 = "incomplete-source-metadata"

	result := Compare(baseline, current, "phase-2-1-valid", semantic.DefaultConfig())
	if !result.HasErrors() || !hasRule(result.Diagnostics, "SIP-COMPAT-001") {
		t.Fatalf("Compare() diagnostics = %#v, want incomplete-source-metadata compatibility error", result.Diagnostics)
	}
}

func comparisonFixtureSet(t testing.TB) *descriptorpb.FileDescriptorSet {
	t.Helper()
	byPath := make(map[string]*descriptorpb.FileDescriptorProto)
	var addFile func(protoreflect.FileDescriptor)
	addFile = func(file protoreflect.FileDescriptor) {
		if _, found := byPath[string(file.Path())]; found {
			return
		}
		byPath[string(file.Path())] = protodesc.ToFileDescriptorProto(file)
		imports := file.Imports()
		for index := range imports.Len() {
			addFile(imports.Get(index).FileDescriptor)
		}
	}
	addFile(testv1alpha1.File_io_github_someip_proto_test_v1alpha1_phase_2_1_schema_fixture_proto)

	paths := make([]string, 0, len(byPath))
	for path := range byPath {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	set := &descriptorpb.FileDescriptorSet{File: make([]*descriptorpb.FileDescriptorProto, 0, len(paths))}
	for _, path := range paths {
		file := byPath[path]
		testutil.CompleteSourceCodeInfo(file)
		set.File = append(set.File, file)
	}
	return set
}

func loadComparisonFixture(t testing.TB, set *descriptorpb.FileDescriptorSet) *descriptor.Set {
	t.Helper()
	input, err := proto.MarshalOptions{Deterministic: true}.Marshal(set)
	if err != nil {
		t.Fatalf("marshal comparison FDS: %v", err)
	}
	loaded, err := descriptor.Load(input, descriptor.DefaultLimits())
	if err != nil {
		t.Fatalf("load comparison FDS: %v", err)
	}
	return loaded
}

func comparisonMethod(t testing.TB, set *descriptorpb.FileDescriptorSet, serviceName, methodName string) *descriptorpb.MethodDescriptorProto {
	t.Helper()
	for _, file := range set.GetFile() {
		for _, service := range file.GetService() {
			if service.GetName() != serviceName {
				continue
			}
			for _, method := range service.GetMethod() {
				if method.GetName() == methodName {
					return method
				}
			}
		}
	}
	t.Fatalf("fixture method %s.%s was not found", serviceName, methodName)
	return nil
}

func comparisonField(t testing.TB, set *descriptorpb.FileDescriptorSet, messageName, fieldName string) *descriptorpb.FieldDescriptorProto {
	t.Helper()
	for _, file := range set.GetFile() {
		for _, message := range file.GetMessageType() {
			if message.GetName() != messageName {
				continue
			}
			for _, field := range message.GetField() {
				if field.GetName() == fieldName {
					return field
				}
			}
		}
	}
	t.Fatalf("fixture field %s.%s was not found", messageName, fieldName)
	return nil
}

func comparisonFile(t testing.TB, set *descriptorpb.FileDescriptorSet, path string) *descriptorpb.FileDescriptorProto {
	t.Helper()
	for _, file := range set.GetFile() {
		if file.GetName() == path {
			return file
		}
	}
	t.Fatalf("fixture file %s was not found", path)
	return nil
}

func comparisonFieldPath(file *descriptorpb.FileDescriptorProto, messageName, fieldName string) ([]int32, bool) {
	for messageIndex, message := range file.GetMessageType() {
		if message.GetName() != messageName {
			continue
		}
		for fieldIndex, field := range message.GetField() {
			if field.GetName() == fieldName {
				return []int32{4, int32(messageIndex), 2, int32(fieldIndex)}, true
			}
		}
	}
	return nil, false
}

func comparisonRootLocation(file *descriptorpb.FileDescriptorProto) *descriptorpb.SourceCodeInfo_Location {
	for _, location := range file.GetSourceCodeInfo().GetLocation() {
		if len(location.GetPath()) == 0 {
			return location
		}
	}
	location := &descriptorpb.SourceCodeInfo_Location{Span: []int32{0, 0, 0}}
	file.SourceCodeInfo.Location = append(file.SourceCodeInfo.Location, location)
	return location
}

func comparisonSourceLocation(file *descriptorpb.FileDescriptorProto, path []int32) *descriptorpb.SourceCodeInfo_Location {
	for _, location := range file.GetSourceCodeInfo().GetLocation() {
		if sourcePathKey(location.GetPath()) == sourcePathKey(path) {
			return location
		}
	}
	return nil
}

func removeSourceLocation(file *descriptorpb.FileDescriptorProto, path []int32) {
	locations := file.GetSourceCodeInfo().GetLocation()
	for index, location := range locations {
		if sourcePathKey(location.GetPath()) != sourcePathKey(path) {
			continue
		}
		file.SourceCodeInfo.Location = append(locations[:index:index], locations[index+1:]...)
		return
	}
}
