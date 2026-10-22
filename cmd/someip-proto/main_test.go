package main

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	manifestv1alpha1 "github.com/eshenhu/someip_proto/gen/go/io/github/someip_proto/manifest/v1alpha1"
	optionsv1alpha1 "github.com/eshenhu/someip_proto/gen/go/io/github/someip_proto/options/v1alpha1"
	testv1alpha1 "github.com/eshenhu/someip_proto/gen/go/io/github/someip_proto/test/v1alpha1"
	"github.com/eshenhu/someip_proto/internal/descriptor"
	"github.com/eshenhu/someip_proto/internal/testutil"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

func TestRun_validatesAndRendersSelectedProfile(t *testing.T) {
	fdsPath := writeFixtureFDS(t)
	manifestPath := filepath.Join(t.TempDir(), "manifest.json")
	reportPath := filepath.Join(t.TempDir(), "report.html")

	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	if code := run([]string{"validate", "--fds", fdsPath, "--profile", "phase-2-1-valid"}, bytes.NewReader(nil), stdout, stderr); code != exitSuccess {
		t.Fatalf("validate exit code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "SIP-UNION-002") || !strings.Contains(stdout.String(), "SIP-LAYOUT-003") {
		t.Fatalf("validate stdout = %q, want fixture warnings", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"manifest", "--fds", fdsPath, "--profile", "phase-2-1-valid", "--format", "json", "--output", manifestPath}, bytes.NewReader(nil), stdout, stderr); code != exitSuccess {
		t.Fatalf("manifest exit code = %d, stderr = %s", code, stderr.String())
	}
	manifestJSON, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest artifact: %v", err)
	}
	manifest := &manifestv1alpha1.Manifest{}
	if err := protojson.Unmarshal(manifestJSON, manifest); err != nil {
		t.Fatalf("unmarshal manifest JSON: %v", err)
	}
	if manifest.GetProfile().GetProfileName() != "phase-2-1-valid" {
		t.Fatalf("rendered profile = %q, want phase-2-1-valid", manifest.GetProfile().GetProfileName())
	}
	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"report", "--fds", fdsPath, "--profile", "phase-2-1-valid", "--format", "html", "--output", reportPath}, bytes.NewReader(nil), stdout, stderr); code != exitSuccess {
		t.Fatalf("report exit code = %d, stderr = %s", code, stderr.String())
	}
	report, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("read HTML report: %v", err)
	}
	if !bytes.Contains(report, []byte("<!doctype html>")) || bytes.Contains(report, []byte("<script")) {
		t.Fatalf("HTML report is not self-contained static HTML: %s", report)
	}
}

func TestRun_validatesBazelProducedDescriptor(t *testing.T) {
	runfiles := os.Getenv("RUNFILES_DIR")
	workspace := os.Getenv("TEST_WORKSPACE")
	if runfiles == "" || workspace == "" {
		t.Skip("Bazel runfiles are unavailable outside the Bazel test environment")
	}
	fdsPath := filepath.Join(runfiles, workspace, "proto", "phase_2_1_valid_descriptor.fds.binpb")
	if _, err := os.Stat(fdsPath); err != nil {
		t.Fatalf("Bazel descriptor runfile %q is unavailable: %v", fdsPath, err)
	}
	assertBazelAlignmentWarningLocation(t, fdsPath)
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	if code := run([]string{"validate", "--fds", fdsPath, "--profile", "phase-2-1-valid"}, bytes.NewReader(nil), stdout, stderr); code != exitSuccess {
		t.Fatalf("validate Bazel descriptor exit code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "SIP-UNION-002") {
		t.Fatalf("validate Bazel descriptor output = %q, want fixture warning", stdout.String())
	}
	if !strings.Contains(stdout.String(), "SIP-LAYOUT-003") || !strings.Contains(stdout.String(), "phase_2_1_schema_fixture.proto:148:3") || !strings.Contains(stdout.String(), "AlignmentWarningPayload.following_word") {
		t.Fatalf("validate Bazel descriptor output = %q, want actionable alignment warning", stdout.String())
	}
}

func assertBazelAlignmentWarningLocation(t *testing.T, fdsPath string) {
	t.Helper()
	input, err := os.ReadFile(fdsPath)
	if err != nil {
		t.Fatalf("read Bazel descriptor: %v", err)
	}
	loaded, err := descriptor.Load(input, descriptor.DefaultLimits())
	if err != nil {
		t.Fatalf("load Bazel descriptor: %v", err)
	}
	value, err := loaded.Files.FindDescriptorByName("io.github.someip_proto.test.v1alpha1.AlignmentWarningPayload.following_word")
	if err != nil {
		t.Fatalf("find alignment field: %v", err)
	}
	field, ok := value.(protoreflect.FieldDescriptor)
	if !ok {
		t.Fatalf("alignment warning descriptor = %T, want field", value)
	}
	if location := loaded.Locations.Path(string(field.ParentFile().Path()), []int32{4, 3, 2, 1}); location.Line != 148 || location.Column != 3 {
		t.Fatalf("raw FDS location = %#v, want line 148, column 3", location)
	}
	if location := loaded.Locations.Descriptor(field); location.Line != 148 || location.Column != 3 {
		t.Fatalf("descriptor location = %#v, want line 148, column 3", location)
	}
}

func TestRun_blocksArtifactOnSemanticFailure(t *testing.T) {
	fdsPath := writeInvalidProfileFixtureFDS(t)
	outputPath := filepath.Join(t.TempDir(), "blocked.binpb")
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}

	code := run([]string{"manifest", "--fds", fdsPath, "--profile", "phase-2-1-invalid", "--output", outputPath}, bytes.NewReader(nil), stdout, stderr)
	if code != exitSemantic {
		t.Fatalf("manifest invalid-profile exit code = %d, stderr = %s", code, stderr.String())
	}
	if _, err := os.Stat(outputPath); !os.IsNotExist(err) {
		t.Fatalf("semantic failure created output artifact: %v", err)
	}
}

func TestRun_blocksArtifactOnCompatibilityFailure(t *testing.T) {
	baselinePath := writeFixtureFDS(t)
	currentPath := writeIncompatibleFixtureFDS(t)
	outputPath := filepath.Join(t.TempDir(), "blocked.md")
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}

	code := run([]string{
		"compare",
		"--baseline", baselinePath,
		"--current", currentPath,
		"--profile", "phase-2-1-valid",
		"--output", outputPath,
	}, bytes.NewReader(nil), stdout, stderr)
	if code != exitSemantic {
		t.Fatalf("compare incompatible-input exit code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "SIP-COMPAT-001") {
		t.Fatalf("compare stderr = %q, want compatibility diagnostic", stderr.String())
	}
	if _, err := os.Stat(outputPath); !os.IsNotExist(err) {
		t.Fatalf("compatibility failure created output artifact: %v", err)
	}
}

func TestRun_classifiesMalformedInputAndComparison(t *testing.T) {
	temporary := t.TempDir()
	malformedPath := filepath.Join(temporary, "malformed.fds.binpb")
	if err := os.WriteFile(malformedPath, []byte("not a descriptor set"), 0o644); err != nil {
		t.Fatalf("write malformed FDS: %v", err)
	}
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	if code := run([]string{"validate", "--fds", malformedPath}, bytes.NewReader(nil), stdout, stderr); code != exitUsage {
		t.Fatalf("malformed validate exit code = %d, stderr = %s", code, stderr.String())
	}

	fdsPath := writeFixtureFDS(t)
	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"compare", "--baseline", fdsPath, "--current", fdsPath, "--profile", "phase-2-1-valid", "--format", "text"}, bytes.NewReader(nil), stdout, stderr); code != exitSuccess {
		t.Fatalf("same-input compare exit code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "SIP-UNION-002") {
		t.Fatalf("compare stdout = %q, want carried normalization warnings", stdout.String())
	}
}

func TestRun_helpDoesNotRequireInput(t *testing.T) {
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	if code := run([]string{"manifest", "--help"}, bytes.NewReader(nil), stdout, stderr); code != exitSuccess {
		t.Fatalf("manifest --help exit code = %d, stderr = %s", code, stderr.String())
	}
}

func TestValidateLimits_rejectsUnreadableMaximumInputSize(t *testing.T) {
	limits := descriptor.DefaultLimits()
	limits.MaxInputBytes = int(^uint(0) >> 1)
	if err := validateLimits(limits); err == nil {
		t.Fatal("validateLimits() error = nil, want input-limit sentinel overflow rejection")
	}
}

func writeFixtureFDS(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fixture.fds.binpb")
	if err := os.WriteFile(path, fixtureDescriptorSet(t,
		testv1alpha1.File_io_github_someip_proto_test_v1alpha1_phase_2_1_schema_fixture_proto,
	), 0o644); err != nil {
		t.Fatalf("write fixture FDS: %v", err)
	}
	return path
}

func writeInvalidProfileFixtureFDS(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "invalid-profile.fds.binpb")
	if err := os.WriteFile(path, fixtureDescriptorSet(t,
		testv1alpha1.File_io_github_someip_proto_test_v1alpha1_phase_2_1_schema_fixture_proto,
		testv1alpha1.File_io_github_someip_proto_test_v1alpha1_phase_2_1_invalid_fixture_proto,
	), 0o644); err != nil {
		t.Fatalf("write invalid profile fixture FDS: %v", err)
	}
	return path
}

func writeIncompatibleFixtureFDS(t *testing.T) string {
	t.Helper()
	set := &descriptorpb.FileDescriptorSet{}
	if err := proto.Unmarshal(fixtureDescriptorSet(t,
		testv1alpha1.File_io_github_someip_proto_test_v1alpha1_phase_2_1_schema_fixture_proto,
	), set); err != nil {
		t.Fatalf("unmarshal fixture FDS: %v", err)
	}
	for _, file := range set.GetFile() {
		for _, message := range file.GetMessageType() {
			if message.GetName() != "PhaseTwoOneV1ServiceSetRequest" {
				continue
			}
			for _, field := range message.GetField() {
				if field.GetName() != "value" {
					continue
				}
				options, ok := proto.GetExtension(field.GetOptions(), optionsv1alpha1.E_Field).(*optionsv1alpha1.FieldOptionsSpec)
				if !ok || options == nil {
					t.Fatal("fixture request value field lacks SOME/IP field options")
				}
				options = proto.Clone(options).(*optionsv1alpha1.FieldOptionsSpec)
				options.ScalarRepresentation = optionsv1alpha1.ScalarRepresentation_SCALAR_REPRESENTATION_UNSIGNED_INTEGER_16
				proto.SetExtension(field.GetOptions(), optionsv1alpha1.E_Field, options)
				encoded, err := proto.MarshalOptions{Deterministic: true}.Marshal(set)
				if err != nil {
					t.Fatalf("marshal incompatible fixture FDS: %v", err)
				}
				path := filepath.Join(t.TempDir(), "incompatible.fds.binpb")
				if err := os.WriteFile(path, encoded, 0o644); err != nil {
					t.Fatalf("write incompatible fixture FDS: %v", err)
				}
				return path
			}
		}
	}
	t.Fatal("fixture request value field was not found")
	return ""
}

func fixtureDescriptorSet(t *testing.T, files ...protoreflect.FileDescriptor) []byte {
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
	for _, file := range files {
		addFile(file)
	}
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
	encoded, err := proto.MarshalOptions{Deterministic: true}.Marshal(set)
	if err != nil {
		t.Fatalf("marshal fixture FDS: %v", err)
	}
	return encoded
}
