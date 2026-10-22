package descriptor

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"sort"
	"testing"

	testv1alpha1 "github.com/eshenhu/someip_proto/gen/go/io/github/someip_proto/test/v1alpha1"
	"github.com/eshenhu/someip_proto/internal/testutil"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

func TestLoad_reconstructsCompleteSourceAwareSet(t *testing.T) {
	input := fixtureDescriptorSet(t, testv1alpha1.File_io_github_someip_proto_test_v1alpha1_phase_2_1_schema_fixture_proto)

	loaded, err := Load(input, DefaultLimits())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	file, found := loaded.FileByPath("io/github/someip_proto/test/v1alpha1/phase_2_1_schema_fixture.proto")
	if !found {
		t.Fatal("Load() did not reconstruct fixture file")
	}
	if options := FileOptions(file); options.GetDeploymentProfile().GetProfileName() != "phase-2-1-valid" {
		t.Fatalf("FileOptions() profile name = %q, want phase-2-1-valid", options.GetDeploymentProfile().GetProfileName())
	}
	if location := loaded.Locations.Descriptor(file.Services().Get(0)); location.ImportPath == "" || location.Line == 0 {
		t.Fatalf("Descriptor() location = %#v, want logical source location", location)
	}
}

func TestLocations_DescriptorResolvesFieldDeclarationPath(t *testing.T) {
	set := descriptorSet(t, testv1alpha1.File_io_github_someip_proto_test_v1alpha1_phase_2_1_schema_fixture_proto)
	completeFixtureSourceInfo(set)
	setFixtureSourceSpan(t, set, "io/github/someip_proto/test/v1alpha1/phase_2_1_schema_fixture.proto", []int32{4, 3, 2, 1}, []int32{139, 2, 139, 16})
	input, err := proto.MarshalOptions{Deterministic: true}.Marshal(set)
	if err != nil {
		t.Fatalf("marshal descriptor set: %v", err)
	}
	loaded, err := Load(input, DefaultLimits())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	descriptor, err := loaded.Files.FindDescriptorByName("io.github.someip_proto.test.v1alpha1.AlignmentWarningPayload.following_word")
	if err != nil {
		t.Fatalf("FindDescriptorByName() error = %v", err)
	}
	field, ok := descriptor.(protoreflect.FieldDescriptor)
	if !ok {
		t.Fatalf("descriptor = %T, want field", descriptor)
	}
	path, found := descriptorSourcePath(field)
	if !found {
		t.Fatal("descriptorSourcePath() did not resolve following_word")
	}
	if got, want := fmt.Sprint(path), "[4 3 2 1]"; got != want {
		t.Fatalf("descriptorSourcePath() = %s, want %s", got, want)
	}
	if location := loaded.Locations.Descriptor(field); location.Line != 140 || location.Column != 3 {
		t.Fatalf("Descriptor() location = %#v, want line 140, column 3", location)
	}
}

func TestLoad_rejectsMissingDependency(t *testing.T) {
	set := descriptorSet(t, testv1alpha1.File_io_github_someip_proto_test_v1alpha1_phase_2_1_schema_fixture_proto)
	set.File[0].Dependency = append(set.File[0].Dependency, "missing.proto")

	input, err := proto.Marshal(set)
	if err != nil {
		t.Fatalf("proto.Marshal() error = %v", err)
	}
	_, err = Load(input, DefaultLimits())
	if !IsInputError(err) {
		t.Fatalf("Load() error = %v, want input error", err)
	}
}

func TestLoad_rejectsMismatchedActiveOptionsAPI(t *testing.T) {
	set := descriptorSet(t, testv1alpha1.File_io_github_someip_proto_test_v1alpha1_phase_2_1_schema_fixture_proto)
	completeFixtureSourceInfo(set)
	for _, file := range set.GetFile() {
		if file.GetName() != "io/github/someip_proto/options/v1alpha1/someip_options.proto" {
			continue
		}
		file.Package = proto.String("io.github.someip_proto.options.mismatched")
		input, err := proto.Marshal(set)
		if err != nil {
			t.Fatalf("marshal descriptor set: %v", err)
		}
		_, err = Load(input, DefaultLimits())
		assertMalformedError(t, err)
		return
	}
	t.Fatal("fixture descriptor closure lacks the options API descriptor")
}

func TestLoad_rejectsMismatchedActiveOptionsAPIBundleMember(t *testing.T) {
	set := descriptorSet(t, testv1alpha1.File_io_github_someip_proto_test_v1alpha1_phase_2_1_schema_fixture_proto)
	completeFixtureSourceInfo(set)
	const importPath = "io/github/someip_proto/options/v1alpha1/someip_sd_options.proto"
	for _, file := range set.GetFile() {
		if file.GetName() != importPath {
			continue
		}
		file.Package = proto.String("io.github.someip_proto.options.mismatched")
		input, err := proto.Marshal(set)
		if err != nil {
			t.Fatalf("marshal descriptor set: %v", err)
		}
		_, err = Load(input, DefaultLimits())
		assertMalformedError(t, err)
		return
	}
	t.Fatalf("fixture descriptor closure lacks %q", importPath)
}

func TestLoad_rejectsMissingActiveOptionsAPIBundleMember(t *testing.T) {
	set := descriptorSet(t, testv1alpha1.File_io_github_someip_proto_test_v1alpha1_phase_2_1_schema_fixture_proto)
	completeFixtureSourceInfo(set)
	const importPath = "io/github/someip_proto/options/v1alpha1/someip_tp_options.proto"
	retained := make([]*descriptorpb.FileDescriptorProto, 0, len(set.GetFile())-1)
	for _, file := range set.GetFile() {
		if file.GetName() == importPath {
			continue
		}
		retained = append(retained, file)
	}
	set.File = retained
	input, err := proto.Marshal(set)
	if err != nil {
		t.Fatalf("marshal descriptor set: %v", err)
	}
	_, err = Load(input, DefaultLimits())
	assertMalformedError(t, err)
}

func TestLoad_rejectsMissingSourceCodeInfo(t *testing.T) {
	set := descriptorSet(t, testv1alpha1.File_io_github_someip_proto_test_v1alpha1_phase_2_1_schema_fixture_proto)
	set.File[0].SourceCodeInfo = nil

	input, err := proto.Marshal(set)
	if err != nil {
		t.Fatalf("proto.Marshal() error = %v", err)
	}
	_, err = Load(input, DefaultLimits())
	if !IsInputError(err) {
		t.Fatalf("Load() error = %v, want input error", err)
	}
}

func TestLoad_rejectsNonLogicalImportPath(t *testing.T) {
	for _, importPath := range []string{"/tmp/fixture.proto", "../fixture.proto", "dir/../fixture.proto", `dir\fixture.proto`, "C:/fixture.proto", "file:///fixture.proto"} {
		t.Run(importPath, func(t *testing.T) {
			set := descriptorSet(t, testv1alpha1.File_io_github_someip_proto_test_v1alpha1_phase_2_1_schema_fixture_proto)
			set.File[0].Name = proto.String(importPath)
			completeFixtureSourceInfo(set)

			input, err := proto.Marshal(set)
			if err != nil {
				t.Fatalf("proto.Marshal() error = %v", err)
			}
			_, err = Load(input, DefaultLimits())
			if !IsInputError(err) {
				t.Fatalf("Load() error = %v, want input error", err)
			}
		})
	}
}

func TestLoad_acceptsRepeatedSourceCodeInfoPath(t *testing.T) {
	input, err := proto.Marshal(&descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{{
		Name:   proto.String("repeated_source_info.proto"),
		Syntax: proto.String("proto3"),
		SourceCodeInfo: &descriptorpb.SourceCodeInfo{Location: []*descriptorpb.SourceCodeInfo_Location{
			{Span: []int32{0, 0, 0}},
			{Path: []int32{8}, Span: []int32{1, 0, 0}},
			{Path: []int32{8}, Span: []int32{0, 0, 0}},
		}},
	}}})
	if err != nil {
		t.Fatalf("marshal descriptor set: %v", err)
	}
	loaded, err := Load(input, DefaultLimits())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if location := loaded.Locations.FileOptions("repeated_source_info.proto"); location.Line != 1 {
		t.Fatalf("repeated source path location = %#v, want earliest source location", location)
	}
}

func TestLoad_classifiesDescriptorOrigins(t *testing.T) {
	newFile := func(name string) *descriptorpb.FileDescriptorProto {
		return &descriptorpb.FileDescriptorProto{
			Name:           proto.String(name),
			Syntax:         proto.String("proto3"),
			SourceCodeInfo: &descriptorpb.SourceCodeInfo{Location: []*descriptorpb.SourceCodeInfo_Location{{Span: []int32{0, 0, 0}}}},
		}
	}
	input, err := proto.Marshal(&descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{
		newFile("io/github/someip_proto/local.proto"),
		newFile("google/protobuf/empty.proto"),
		newFile("example/dependency.proto"),
	}})
	if err != nil {
		t.Fatalf("marshal descriptor set: %v", err)
	}
	loaded, err := Load(input, DefaultLimits())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	for importPath, want := range map[string]FileOrigin{
		"io/github/someip_proto/local.proto": FileOriginLocal,
		"google/protobuf/empty.proto":        FileOriginBuiltin,
		"example/dependency.proto":           FileOriginUnknown,
	} {
		if got := loaded.OriginByPath(importPath); got != want {
			t.Fatalf("OriginByPath(%q) = %q, want %q", importPath, got, want)
		}
	}
}

func TestLoad_classifiesOnlyExactLockedDependencyDescriptors(t *testing.T) {
	const importPath = "example/locked_dependency.proto"
	file := &descriptorpb.FileDescriptorProto{
		Name:           proto.String(importPath),
		Package:        proto.String("example.locked"),
		Syntax:         proto.String("proto3"),
		SourceCodeInfo: &descriptorpb.SourceCodeInfo{Location: []*descriptorpb.SourceCodeInfo_Location{{Span: []int32{0, 0, 0}}}},
	}
	encodedFile, err := proto.MarshalOptions{Deterministic: true}.Marshal(file)
	if err != nil {
		t.Fatalf("marshal dependency descriptor: %v", err)
	}
	sum := sha256.Sum256(encodedFile)
	originalLedger := lockedDependencyFiles
	lockedDependencyFiles = map[string]lockedDependencyFile{
		importPath: {
			sha256: fmt.Sprintf("%x", sum),
			module: "buf.build/example/locked",
			commit: "deadbeef",
			digest: "b5:test",
		},
	}
	t.Cleanup(func() { lockedDependencyFiles = originalLedger })

	load := func(file *descriptorpb.FileDescriptorProto) *Set {
		t.Helper()
		input, err := proto.Marshal(&descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{file}})
		if err != nil {
			t.Fatalf("marshal descriptor set: %v", err)
		}
		loaded, err := Load(input, DefaultLimits())
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		return loaded
	}
	if got := load(file).OriginByPath(importPath); got != FileOriginLockedDependency {
		t.Fatalf("exact dependency origin = %q, want %q", got, FileOriginLockedDependency)
	}
	tampered := proto.Clone(file).(*descriptorpb.FileDescriptorProto)
	tampered.Package = proto.String("example.tampered")
	if got := load(tampered).OriginByPath(importPath); got != FileOriginUnknown {
		t.Fatalf("tampered dependency origin = %q, want %q", got, FileOriginUnknown)
	}
}

func TestLoad_rejectsMissingExactDeclarationSourceLocation(t *testing.T) {
	file := &descriptorpb.FileDescriptorProto{
		Name:   proto.String("missing_declaration_location.proto"),
		Syntax: proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{{
			Name: proto.String("Payload"),
		}},
		SourceCodeInfo: &descriptorpb.SourceCodeInfo{
			Location: []*descriptorpb.SourceCodeInfo_Location{{Span: []int32{0, 0, 0}}},
		},
	}
	input, err := proto.Marshal(&descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{file}})
	if err != nil {
		t.Fatalf("marshal descriptor set: %v", err)
	}
	_, err = Load(input, DefaultLimits())
	assertMalformedError(t, err)
}

func TestLoad_rejectsConflictingDeclarationSourceLocations(t *testing.T) {
	file := &descriptorpb.FileDescriptorProto{
		Name:   proto.String("conflicting_declaration_location.proto"),
		Syntax: proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{{
			Name: proto.String("Payload"),
		}},
	}
	testutil.CompleteSourceCodeInfo(file)
	file.SourceCodeInfo.Location = append(file.SourceCodeInfo.Location, &descriptorpb.SourceCodeInfo_Location{
		Path: []int32{4, 0},
		Span: []int32{1, 0, 1, 7},
	})
	input, err := proto.Marshal(&descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{file}})
	if err != nil {
		t.Fatalf("marshal descriptor set: %v", err)
	}
	_, err = Load(input, DefaultLimits())
	assertMalformedError(t, err)
}

func TestLoad_rejectsMalformedSourceCodeInfo(t *testing.T) {
	for name, locations := range map[string][]*descriptorpb.SourceCodeInfo_Location{
		"missing root": {
			{Path: []int32{4, 0}, Span: []int32{0, 0, 0}},
		},
		"negative span": {
			{Span: []int32{-1, 0, 0}},
		},
		"reversed span": {
			{Span: []int32{1, 3, 1, 2}},
		},
		"invalid span length": {
			{Span: []int32{0, 0}},
		},
	} {
		t.Run(name, func(t *testing.T) {
			input, err := proto.Marshal(&descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{{
				Name:           proto.String("source_info.proto"),
				Syntax:         proto.String("proto3"),
				SourceCodeInfo: &descriptorpb.SourceCodeInfo{Location: locations},
			}}})
			if err != nil {
				t.Fatalf("marshal descriptor set: %v", err)
			}
			_, err = Load(input, DefaultLimits())
			assertMalformedError(t, err)
		})
	}
}

func TestLoad_enforcesConfiguredInputLimit(t *testing.T) {
	input := fixtureDescriptorSet(t, testv1alpha1.File_io_github_someip_proto_test_v1alpha1_phase_2_1_schema_fixture_proto)

	_, err := Load(input, Limits{MaxInputBytes: len(input) - 1})
	var inputErr *Error
	if !errors.As(err, &inputErr) || inputErr.Kind != ErrorLimit {
		t.Fatalf("Load() error = %v, want limit error", err)
	}
}

func TestLoad_enforcesConfiguredDescriptorDepthLimit(t *testing.T) {
	file := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("deep.proto"),
		Package: proto.String("fixture"),
		Syntax:  proto.String("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{
			nestedMessageDescriptor(3),
		},
		SourceCodeInfo: &descriptorpb.SourceCodeInfo{
			Location: []*descriptorpb.SourceCodeInfo_Location{{Span: []int32{0, 0, 0}}},
		},
	}
	testutil.CompleteSourceCodeInfo(file)
	input, err := proto.Marshal(&descriptorpb.FileDescriptorSet{File: []*descriptorpb.FileDescriptorProto{file}})
	if err != nil {
		t.Fatalf("proto.Marshal() error = %v", err)
	}
	_, err = Load(input, Limits{MaxDescriptorDepth: 2})
	assertLimitError(t, err)
}

func TestLoad_enforcesConfiguredSourceCodeInfoLimits(t *testing.T) {
	set := descriptorSet(t, testv1alpha1.File_io_github_someip_proto_test_v1alpha1_phase_2_1_schema_fixture_proto)
	completeFixtureSourceInfo(set)
	set.File[0].SourceCodeInfo.Location = append(set.File[0].GetSourceCodeInfo().GetLocation(), &descriptorpb.SourceCodeInfo_Location{
		Path: []int32{4, 0},
		Span: []int32{0, 0, 0},
	})
	input, err := proto.Marshal(set)
	if err != nil {
		t.Fatalf("proto.Marshal() error = %v", err)
	}
	_, err = Load(input, Limits{MaxSourceLocations: 1})
	assertLimitError(t, err)
	_, err = Load(input, Limits{MaxSourcePathElements: 1})
	assertLimitError(t, err)
}

func TestLoad_preservesCompleteUnknownOptionEncoding(t *testing.T) {
	set := descriptorSet(t, testv1alpha1.File_io_github_someip_proto_test_v1alpha1_phase_2_1_schema_fixture_proto)
	var fixtureFile *descriptorpb.FileDescriptorProto
	for _, file := range set.File {
		if file.GetName() == "io/github/someip_proto/test/v1alpha1/phase_2_1_schema_fixture.proto" {
			fixtureFile = file
			break
		}
	}
	if fixtureFile == nil || fixtureFile.GetOptions() == nil {
		t.Fatal("fixture descriptor lacks file options")
	}
	fileOptions := fixtureFile.GetOptions()
	encoded := protowire.AppendTag(nil, 65000, protowire.VarintType)
	encoded = protowire.AppendVarint(encoded, 42)
	fileOptions.ProtoReflect().SetUnknown(encoded)
	completeFixtureSourceInfo(set)
	input, err := proto.MarshalOptions{Deterministic: true}.Marshal(set)
	if err != nil {
		t.Fatalf("marshal descriptor set: %v", err)
	}
	loaded, err := Load(input, DefaultLimits())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	for _, unknown := range loaded.UnknownOptions {
		if unknown.GetFieldNumber() != 65000 {
			continue
		}
		if unknown.GetWireType() != uint32(protowire.VarintType) {
			t.Fatalf("unknown wire type = %d, want %d", unknown.GetWireType(), protowire.VarintType)
		}
		if !bytes.Equal(unknown.GetEncodedField(), encoded) {
			t.Fatalf("unknown encoded field = %x, want %x", unknown.GetEncodedField(), encoded)
		}
		return
	}
	t.Fatalf("Load() unknown options = %#v, want field 65000", loaded.UnknownOptions)
}

func fixtureDescriptorSet(t testing.TB, file protoreflect.FileDescriptor) []byte {
	t.Helper()
	set := descriptorSet(t, file)
	completeFixtureSourceInfo(set)
	input, err := proto.MarshalOptions{Deterministic: true}.Marshal(set)
	if err != nil {
		t.Fatalf("proto.Marshal() error = %v", err)
	}
	return input
}

func descriptorSet(t testing.TB, file protoreflect.FileDescriptor) *descriptorpb.FileDescriptorSet {
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
		set.File = append(set.File, byPath[path])
	}
	return set
}

func completeFixtureSourceInfo(set *descriptorpb.FileDescriptorSet) {
	for _, file := range set.GetFile() {
		testutil.CompleteSourceCodeInfo(file)
	}
}

func setFixtureSourceSpan(t testing.TB, set *descriptorpb.FileDescriptorSet, fileName string, path, span []int32) {
	t.Helper()
	for _, file := range set.GetFile() {
		if file.GetName() != fileName {
			continue
		}
		for _, location := range file.GetSourceCodeInfo().GetLocation() {
			if pathKey(location.GetPath()) != pathKey(path) {
				continue
			}
			location.Span = append([]int32(nil), span...)
			return
		}
	}
	t.Fatalf("fixture source location %q %v was not found", fileName, path)
}

func nestedMessageDescriptor(depth int) *descriptorpb.DescriptorProto {
	name := "Message"
	message := &descriptorpb.DescriptorProto{Name: &name}
	if depth > 1 {
		message.NestedType = []*descriptorpb.DescriptorProto{nestedMessageDescriptor(depth - 1)}
	}
	return message
}

func assertLimitError(t testing.TB, err error) {
	t.Helper()
	var inputErr *Error
	if !errors.As(err, &inputErr) || inputErr.Kind != ErrorLimit {
		t.Fatalf("Load() error = %v, want resource-limit error", err)
	}
}

func assertMalformedError(t testing.TB, err error) {
	t.Helper()
	var inputErr *Error
	if !errors.As(err, &inputErr) || inputErr.Kind != ErrorMalformed {
		t.Fatalf("Load() error = %v, want malformed-input error", err)
	}
}
