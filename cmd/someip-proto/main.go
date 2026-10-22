// someip-proto validates and renders static SOME/IP protobuf contracts.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/eshenhu/someip_proto/compatibility"
	manifestv1alpha1 "github.com/eshenhu/someip_proto/gen/go/io/github/someip_proto/manifest/v1alpha1"
	"github.com/eshenhu/someip_proto/internal/descriptor"
	"github.com/eshenhu/someip_proto/internal/render"
	"github.com/eshenhu/someip_proto/internal/semantic"
	"github.com/eshenhu/someip_proto/internal/version"
)

const (
	exitSuccess  = 0
	exitSemantic = 1
	exitUsage    = 2
	exitInternal = 3
	exitHelp     = -1
)

func main() {
	os.Exit(runSafely(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func runSafely(arguments []string, stdin io.Reader, stdout, stderr io.Writer) (exitCode int) {
	defer func() {
		if recovered := recover(); recovered != nil {
			fmt.Fprintf(stderr, "internal tool failure: %v\n", recovered)
			exitCode = exitInternal
		}
	}()
	return run(arguments, stdin, stdout, stderr)
}

func run(arguments []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(arguments) == 0 {
		printUsage(stderr)
		return exitUsage
	}
	switch arguments[0] {
	case "help", "--help", "-h":
		printUsage(stdout)
		return exitSuccess
	case "validate":
		return runValidate(arguments[1:], stdin, stdout, stderr)
	case "manifest":
		return runManifest(arguments[1:], stdin, stdout, stderr)
	case "report":
		return runReport(arguments[1:], stdin, stdout, stderr)
	case "compare":
		return runCompare(arguments[1:], stdin, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command %q\n", arguments[0])
		printUsage(stderr)
		return exitUsage
	}
}

func runValidate(arguments []string, stdin io.Reader, stdout, stderr io.Writer) int {
	options, flags, code := parseCommonFlags("validate", arguments, stderr)
	if code != exitSuccess {
		return code
	}
	format := flags.String("format", "text", "diagnostic output format: text or json")
	if code := parseFlags(flags, arguments, stderr); code == exitHelp {
		return exitSuccess
	} else if code != exitSuccess {
		return code
	}
	if err := options.validateInput(); err != nil {
		fmt.Fprintln(stderr, err)
		return exitUsage
	}
	if *format != "text" && *format != "json" {
		fmt.Fprintf(stderr, "validate: unsupported --format %q (want text or json)\n", *format)
		return exitUsage
	}
	set, code := loadSet(options.fds, options.limits, stdin, stderr)
	if code != exitSuccess {
		return code
	}
	var results []*semantic.Result
	if options.profile == "" {
		results = semantic.ValidateAll(set, options.config())
	} else {
		results = []*semantic.Result{semantic.Normalize(set, options.profile, options.config())}
	}
	diagnostics := collectDiagnostics(results)
	var output []byte
	var err error
	if *format == "json" {
		output, err = render.JSONDiagnostics(diagnostics)
	} else {
		output = render.TextDiagnostics(diagnostics)
		if len(output) == 0 {
			output = []byte("validation succeeded\n")
		}
	}
	if err != nil {
		fmt.Fprintf(stderr, "render validation diagnostics: %v\n", err)
		return exitInternal
	}
	if code := writeOutput(options.output, output, stdout, stderr); code != exitSuccess {
		return code
	}
	if anyErrors(results) {
		return exitSemantic
	}
	return exitSuccess
}

func runManifest(arguments []string, stdin io.Reader, stdout, stderr io.Writer) int {
	options, flags, code := parseCommonFlags("manifest", arguments, stderr)
	if code != exitSuccess {
		return code
	}
	format := flags.String("format", "binpb", "manifest output format: binpb or json")
	if code := parseFlags(flags, arguments, stderr); code == exitHelp {
		return exitSuccess
	} else if code != exitSuccess {
		return code
	}
	if err := options.validateInput(); err != nil {
		fmt.Fprintln(stderr, err)
		return exitUsage
	}
	if options.profile == "" {
		fmt.Fprintln(stderr, "manifest: --profile is required")
		return exitUsage
	}
	set, code := loadSet(options.fds, options.limits, stdin, stderr)
	if code != exitSuccess {
		return code
	}
	result := semantic.Normalize(set, options.profile, options.config())
	if result.HasErrors() {
		_, _ = stderr.Write(render.TextDiagnostics(result.Diagnostics))
		return exitSemantic
	}
	output, err := renderManifest(*format, result.Manifest)
	if err != nil {
		fmt.Fprintf(stderr, "manifest: %v\n", err)
		return exitUsage
	}
	if code := writeOutput(options.output, output, stdout, stderr); code != exitSuccess {
		return code
	}
	if len(result.Diagnostics) > 0 {
		_, _ = stderr.Write(render.TextDiagnostics(result.Diagnostics))
	}
	return exitSuccess
}

func runReport(arguments []string, stdin io.Reader, stdout, stderr io.Writer) int {
	options, flags, code := parseCommonFlags("report", arguments, stderr)
	if code != exitSuccess {
		return code
	}
	format := flags.String("format", "markdown", "report output format: markdown, html, or json")
	if code := parseFlags(flags, arguments, stderr); code == exitHelp {
		return exitSuccess
	} else if code != exitSuccess {
		return code
	}
	if err := options.validateInput(); err != nil {
		fmt.Fprintln(stderr, err)
		return exitUsage
	}
	if options.profile == "" {
		fmt.Fprintln(stderr, "report: --profile is required")
		return exitUsage
	}
	set, code := loadSet(options.fds, options.limits, stdin, stderr)
	if code != exitSuccess {
		return code
	}
	result := semantic.Normalize(set, options.profile, options.config())
	if result.HasErrors() {
		_, _ = stderr.Write(render.TextDiagnostics(result.Diagnostics))
		return exitSemantic
	}
	output, err := renderReport(*format, result.Manifest)
	if err != nil {
		fmt.Fprintf(stderr, "report: %v\n", err)
		return exitUsage
	}
	if code := writeOutput(options.output, output, stdout, stderr); code != exitSuccess {
		return code
	}
	return exitSuccess
}

func runCompare(arguments []string, stdin io.Reader, stdout, stderr io.Writer) int {
	flags := newFlagSet("compare", stderr)
	baselinePath := flags.String("baseline", "", "baseline source-aware FileDescriptorSet path")
	currentPath := flags.String("current", "", "current source-aware FileDescriptorSet path")
	profile := flags.String("profile", "", "deployment profile name")
	outputPath := flags.String("output", "", "artifact output path; defaults to standard output")
	format := flags.String("format", "markdown", "comparison output format: text, json, markdown, or html")
	toolVersion := flags.String("tool-version", version.ToolVersion, "tool version for artifact provenance")
	sourceRevision := flags.String("source-revision", version.SourceRevision, "optional source revision for artifact provenance")
	limits := descriptor.DefaultLimits()
	addLimitFlags(flags, &limits)
	if code := parseFlags(flags, arguments, stderr); code == exitHelp {
		return exitSuccess
	} else if code != exitSuccess {
		return code
	}
	if *baselinePath == "" || *currentPath == "" || *profile == "" {
		fmt.Fprintln(stderr, "compare: --baseline, --current, and --profile are required")
		return exitUsage
	}
	if err := validateLimits(limits); err != nil {
		fmt.Fprintln(stderr, err)
		return exitUsage
	}
	if *format != "text" && *format != "json" && *format != "markdown" && *format != "html" {
		fmt.Fprintf(stderr, "compare: unsupported --format %q\n", *format)
		return exitUsage
	}
	baseline, code := loadSet(*baselinePath, limits, stdin, stderr)
	if code != exitSuccess {
		return code
	}
	current, code := loadSet(*currentPath, limits, stdin, stderr)
	if code != exitSuccess {
		return code
	}
	result := compatibility.Compare(baseline, current, *profile, semantic.Config{
		ToolVersion:    *toolVersion,
		SourceRevision: *sourceRevision,
	})
	if result.HasErrors() {
		_, _ = stderr.Write(render.TextDiagnostics(result.Diagnostics))
		return exitSemantic
	}
	report := result.ReportManifest()
	var rendered []byte
	var err error
	if *format == "text" {
		rendered = render.TextDiagnostics(result.Diagnostics)
		if len(rendered) == 0 {
			rendered = []byte("compatibility succeeded\n")
		}
	} else {
		rendered, err = renderReport(*format, report)
	}
	if err != nil {
		fmt.Fprintf(stderr, "compare: %v\n", err)
		return exitUsage
	}
	return writeOutput(*outputPath, rendered, stdout, stderr)
}

type commonOptions struct {
	fds            string
	profile        string
	output         string
	toolVersion    string
	sourceRevision string
	limits         descriptor.Limits
}

func parseCommonFlags(command string, arguments []string, stderr io.Writer) (*commonOptions, *flag.FlagSet, int) {
	flags := newFlagSet(command, stderr)
	options := &commonOptions{}
	flags.StringVar(&options.fds, "fds", "", "source-aware FileDescriptorSet path, or - for standard input")
	flags.StringVar(&options.fds, "input", "", "alias for --fds")
	flags.StringVar(&options.fds, "descriptor-set", "", "alias for --fds")
	flags.StringVar(&options.profile, "profile", "", "deployment profile name")
	flags.StringVar(&options.output, "output", "", "artifact output path; defaults to standard output")
	flags.StringVar(&options.toolVersion, "tool-version", version.ToolVersion, "tool version for artifact provenance")
	flags.StringVar(&options.sourceRevision, "source-revision", version.SourceRevision, "optional source revision for artifact provenance")
	options.limits = descriptor.DefaultLimits()
	addLimitFlags(flags, &options.limits)
	return options, flags, exitSuccess
}

func (o *commonOptions) validateInput() error {
	if o.fds == "" {
		return errors.New("--fds is required")
	}
	return validateLimits(o.limits)
}

func (o *commonOptions) config() semantic.Config {
	return semantic.Config{ToolVersion: o.toolVersion, SourceRevision: o.sourceRevision}
}

func newFlagSet(name string, stderr io.Writer) *flag.FlagSet {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() { printCommandUsage(stderr, name) }
	return flags
}

func parseFlags(flags *flag.FlagSet, arguments []string, stderr io.Writer) int {
	if err := flags.Parse(arguments); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitHelp
		}
		return exitUsage
	}
	if len(flags.Args()) != 0 {
		fmt.Fprintf(stderr, "%s: unexpected positional arguments: %s\n", flags.Name(), strings.Join(flags.Args(), " "))
		return exitUsage
	}
	return exitSuccess
}

func addLimitFlags(flags *flag.FlagSet, limits *descriptor.Limits) {
	defaults := descriptor.DefaultLimits()
	limits.MaxInputBytes = defaults.MaxInputBytes
	limits.MaxFiles = defaults.MaxFiles
	limits.MaxDescriptors = defaults.MaxDescriptors
	limits.MaxDescriptorDepth = defaults.MaxDescriptorDepth
	limits.MaxSourceLocations = defaults.MaxSourceLocations
	limits.MaxSourcePathElements = defaults.MaxSourcePathElements
	flags.IntVar(&limits.MaxInputBytes, "max-input-bytes", defaults.MaxInputBytes, "maximum FDS input bytes")
	flags.IntVar(&limits.MaxFiles, "max-files", defaults.MaxFiles, "maximum FDS files")
	flags.IntVar(&limits.MaxDescriptors, "max-descriptors", defaults.MaxDescriptors, "maximum FDS descriptors")
	flags.IntVar(&limits.MaxDescriptorDepth, "max-descriptor-depth", defaults.MaxDescriptorDepth, "maximum nested descriptor depth")
	flags.IntVar(&limits.MaxSourceLocations, "max-source-locations", defaults.MaxSourceLocations, "maximum FDS SourceCodeInfo locations")
	flags.IntVar(&limits.MaxSourcePathElements, "max-source-path-elements", defaults.MaxSourcePathElements, "maximum SourceCodeInfo path elements")
}

func validateLimits(limits descriptor.Limits) error {
	if limits.MaxInputBytes <= 0 || limits.MaxFiles <= 0 || limits.MaxDescriptors <= 0 || limits.MaxDescriptorDepth <= 0 || limits.MaxSourceLocations <= 0 || limits.MaxSourcePathElements <= 0 {
		return errors.New("all FDS resource limits must be positive")
	}
	if limits.MaxInputBytes == int(^uint(0)>>1) {
		return errors.New("max-input-bytes must leave room for the input-limit sentinel")
	}
	return nil
}

func loadSet(path string, limits descriptor.Limits, stdin io.Reader, stderr io.Writer) (*descriptor.Set, int) {
	input, err := readInput(path, limits.MaxInputBytes, stdin)
	if err != nil {
		fmt.Fprintf(stderr, "read FDS input: %v\n", err)
		return nil, exitUsage
	}
	set, err := descriptor.Load(input, limits)
	if err != nil {
		fmt.Fprintf(stderr, "invalid FDS input: %v\n", err)
		return nil, exitUsage
	}
	return set, exitSuccess
}

func readInput(path string, maximumBytes int, stdin io.Reader) ([]byte, error) {
	if maximumBytes <= 0 || maximumBytes == int(^uint(0)>>1) {
		return nil, errors.New("maximum input bytes must leave room for the input-limit sentinel")
	}
	var reader io.Reader
	var closeInput func() error
	if path == "-" {
		reader = stdin
	} else {
		file, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		reader = file
		closeInput = file.Close
	}
	if closeInput != nil {
		defer func() { _ = closeInput() }()
	}
	input, err := io.ReadAll(io.LimitReader(reader, int64(maximumBytes)+1))
	if err != nil {
		return nil, err
	}
	if len(input) > maximumBytes {
		return nil, fmt.Errorf("input exceeds the %d-byte limit", maximumBytes)
	}
	return input, nil
}

func renderManifest(format string, manifest *manifestv1alpha1.Manifest) ([]byte, error) {
	switch format {
	case "binpb", "binary":
		return render.Binary(manifest)
	case "json":
		return render.JSON(manifest)
	default:
		return nil, fmt.Errorf("unsupported --format %q (want binpb or json)", format)
	}
}

func renderReport(format string, manifest *manifestv1alpha1.Manifest) ([]byte, error) {
	switch format {
	case "markdown", "md":
		return render.Markdown(manifest)
	case "html":
		return render.HTML(manifest)
	case "json":
		return render.JSON(manifest)
	default:
		return nil, fmt.Errorf("unsupported --format %q (want markdown, html, or json)", format)
	}
}

func writeOutput(path string, contents []byte, stdout, stderr io.Writer) int {
	if path == "" || path == "-" {
		if _, err := stdout.Write(contents); err != nil {
			fmt.Fprintf(stderr, "write standard output: %v\n", err)
			return exitInternal
		}
		return exitSuccess
	}
	if err := render.WriteFileAtomic(path, contents, 0o644); err != nil {
		fmt.Fprintf(stderr, "write artifact: %v\n", err)
		return exitInternal
	}
	return exitSuccess
}

func collectDiagnostics(results []*semantic.Result) []*manifestv1alpha1.Diagnostic {
	var diagnostics []*manifestv1alpha1.Diagnostic
	for _, result := range results {
		if result != nil {
			diagnostics = append(diagnostics, result.Diagnostics...)
		}
	}
	return diagnostics
}

func anyErrors(results []*semantic.Result) bool {
	for _, result := range results {
		if result != nil && result.HasErrors() {
			return true
		}
	}
	return false
}

func printUsage(writer io.Writer) {
	fmt.Fprint(writer, `usage: someip-proto <command> [flags]

Commands:
  validate  Validate one selected profile or every profile in an FDS.
  manifest  Write a selected profile's canonical manifest (binpb or JSON).
  report    Write a selected profile's Markdown, HTML, or JSON report.
  compare   Compare selected baseline/current profiles from two FDS inputs.

Exit codes: 0 success, 1 semantic or compatibility failure,
2 invalid usage or malformed input, 3 internal/tool failure.
`)
}

func printCommandUsage(writer io.Writer, command string) {
	fmt.Fprintf(writer, "usage: someip-proto %s [flags]\n", command)
}
