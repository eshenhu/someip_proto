package compatibility

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eshenhu/someip_proto/internal/descriptor"
	"github.com/eshenhu/someip_proto/internal/render"
	"github.com/eshenhu/someip_proto/internal/semantic"
)

var baselineArtifactNames = []string{
	"baseline.fds.binpb",
	"manifest.binpb",
	"manifest.json",
	"report.html",
	"report.md",
}

type baselineSuite struct {
	name               string
	profile            string
	descriptorArtifact string
	relativeDirectory  string
}

var smartWorkshopLightingBaselineSuites = []baselineSuite{
	{
		name:               "core",
		profile:            "smart-workshop-lighting",
		descriptorArtifact: "smart_workshop_lighting_descriptor.fds.binpb",
	},
	{
		name:               "sd-tp",
		profile:            "smart-workshop-lighting-sd-tp",
		descriptorArtifact: "smart_workshop_lighting_sd_tp_descriptor.fds.binpb",
		relativeDirectory:  "sd-tp",
	},
}

func TestV010SmartWorkshopLightingBaselines(t *testing.T) {
	root := bazelRunfilesRoot(t)
	if root == "" {
		t.Skip("Bazel runfiles are unavailable outside the Bazel test environment")
	}
	for _, suite := range smartWorkshopLightingBaselineSuites {
		t.Run(suite.name, func(t *testing.T) {
			testV010SmartWorkshopLightingBaselineSuite(t, root, suite)
		})
	}
}

func testV010SmartWorkshopLightingBaselineSuite(t *testing.T, root string, suite baselineSuite) {
	t.Helper()
	baselineDirectory := filepath.Join(root, "compatibility", "baselines", "v0.1.0", "smart-workshop-lighting", suite.relativeDirectory)
	baselineInput := readBaselineFile(t, filepath.Join(baselineDirectory, "baseline.fds.binpb"))
	currentInput := readBaselineFile(t, filepath.Join(root, "proto", suite.descriptorArtifact))
	if !bytes.Equal(baselineInput, currentInput) {
		t.Fatalf("SmartWorkshopLighting %s FDS differs from the v0.1.0 compatibility baseline", suite.name)
	}

	baseline, err := descriptor.Load(baselineInput, descriptor.DefaultLimits())
	if err != nil {
		t.Fatalf("load baseline FDS: %v", err)
	}
	current, err := descriptor.Load(currentInput, descriptor.DefaultLimits())
	if err != nil {
		t.Fatalf("load current FDS: %v", err)
	}
	config := semantic.Config{ToolVersion: "v0.1.0", SourceRevision: "v0.1.0"}
	result := semantic.Normalize(current, suite.profile, config)
	if result.HasErrors() {
		t.Fatalf("normalize SmartWorkshopLighting %s: %v", suite.name, result.Diagnostics)
	}
	binaryManifest, err := render.Binary(result.Manifest)
	assertGoldenBytes(t, filepath.Join(baselineDirectory, "manifest.binpb"), binaryManifest, err)
	jsonManifest, err := render.JSON(result.Manifest)
	assertGoldenBytes(t, filepath.Join(baselineDirectory, "manifest.json"), jsonManifest, err)
	htmlReport, err := render.HTML(result.Manifest)
	assertGoldenBytes(t, filepath.Join(baselineDirectory, "report.html"), htmlReport, err)
	markdownReport, err := render.Markdown(result.Manifest)
	assertGoldenBytes(t, filepath.Join(baselineDirectory, "report.md"), markdownReport, err)

	comparison := Compare(baseline, current, suite.profile, config)
	if comparison.HasErrors() {
		t.Fatalf("v0.1.0 %s baseline is incompatible with itself: %v", suite.name, comparison.Diagnostics)
	}
	verifyGoldenChecksums(t, baselineDirectory)
}

func bazelRunfilesRoot(t *testing.T) string {
	t.Helper()
	runfiles := os.Getenv("RUNFILES_DIR")
	workspace := os.Getenv("TEST_WORKSPACE")
	if runfiles == "" || workspace == "" {
		return ""
	}
	return filepath.Join(runfiles, workspace)
}

func readBaselineFile(t *testing.T, path string) []byte {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %q: %v", path, err)
	}
	if bytes.Contains(contents, []byte("/home/")) {
		t.Fatalf("artifact %q contains an absolute local path", path)
	}
	return contents
}

func assertGoldenBytes(t *testing.T, path string, actual []byte, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("render %q: %v", filepath.Base(path), err)
	}
	expected := readBaselineFile(t, path)
	if !bytes.Equal(expected, actual) {
		t.Fatalf("golden %q is not reproducible: %s", path, firstByteDifference(expected, actual))
	}
}

func firstByteDifference(expected, actual []byte) string {
	limit := len(expected)
	if len(actual) < limit {
		limit = len(actual)
	}
	for index := 0; index < limit; index++ {
		if expected[index] == actual[index] {
			continue
		}
		start := index - 48
		if start < 0 {
			start = 0
		}
		end := index + 48
		if end > len(expected) {
			end = len(expected)
		}
		actualEnd := end
		if actualEnd > len(actual) {
			actualEnd = len(actual)
		}
		return fmt.Sprintf("byte %d: expected %q, actual %q", index, expected[start:end], actual[start:actualEnd])
	}
	return fmt.Sprintf("length mismatch: expected %d bytes, actual %d bytes", len(expected), len(actual))
}

func verifyGoldenChecksums(t *testing.T, directory string) {
	t.Helper()
	contents := readBaselineFile(t, filepath.Join(directory, "SHA256SUMS"))
	lines := strings.Split(strings.TrimSuffix(string(contents), "\n"), "\n")
	if len(lines) != len(baselineArtifactNames) {
		t.Fatalf("SHA256SUMS has %d entries, want %d", len(lines), len(baselineArtifactNames))
	}
	for index, name := range baselineArtifactNames {
		parts := strings.Fields(lines[index])
		if len(parts) != 2 || parts[1] != name {
			t.Fatalf("SHA256SUMS entry %d = %q, want checksum for %q", index, lines[index], name)
		}
		digest := sha256.Sum256(readBaselineFile(t, filepath.Join(directory, name)))
		if parts[0] != fmt.Sprintf("%x", digest) {
			t.Fatalf("SHA256SUMS digest for %q is stale", name)
		}
	}
}
