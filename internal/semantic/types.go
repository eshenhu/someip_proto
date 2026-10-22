// Package semantic validates annotated descriptor sets and materializes the
// normalized manifest contract.
package semantic

import (
	"sort"

	manifestv1alpha1 "github.com/eshenhu/someip_proto/gen/go/io/github/someip_proto/manifest/v1alpha1"
	optionsv1alpha1 "github.com/eshenhu/someip_proto/gen/go/io/github/someip_proto/options/v1alpha1"
	"github.com/eshenhu/someip_proto/internal/descriptor"
	"google.golang.org/protobuf/reflect/protoreflect"
)

const normativeSourceCommit = "94e4ea932e50273897a014bc57f7ae89f1816b84"

// Config controls deterministic normalization provenance.
type Config struct {
	// ToolVersion identifies the executable emitting the artifact.
	ToolVersion string
	// SourceRevision is optional caller-supplied source provenance.
	SourceRevision string
}

// DefaultConfig returns the default local-tool provenance.
func DefaultConfig() Config {
	return Config{ToolVersion: "devel"}
}

// Result contains a normalized manifest and all semantic diagnostics.
type Result struct {
	Manifest    *manifestv1alpha1.Manifest
	Diagnostics []*manifestv1alpha1.Diagnostic
}

// HasErrors reports whether semantic validation produced an error diagnostic.
func (r *Result) HasErrors() bool {
	for _, diagnostic := range r.Diagnostics {
		if diagnostic.GetSeverity() == manifestv1alpha1.DiagnosticSeverity_DIAGNOSTIC_SEVERITY_ERROR {
			return true
		}
	}
	return false
}

type profileDefinition struct {
	file       protoreflect.FileDescriptor
	definition *optionsv1alpha1.DeploymentProfile
}

type normalizer struct {
	set                       *descriptor.Set
	config                    Config
	diagnostics               diagnosticCollector
	manifest                  *manifestv1alpha1.Manifest
	services                  map[protoreflect.FullName]*serviceInfo
	providerBindings          []*providerBindingInfo
	reportedUnapprovedOrigins map[string]struct{}
}

// providerBindingInfo connects an authored provider binding to its resolved
// service, endpoint collection, and manifest record. It is kept only while a
// profile is normalized; the manifest remains the public output boundary.
type providerBindingInfo struct {
	provider         *optionsv1alpha1.ProviderInstance
	providerManifest *manifestv1alpha1.ProviderInstanceManifest
	binding          *optionsv1alpha1.ServiceVersionBinding
	bindingManifest  *manifestv1alpha1.ProviderServiceVersionBindingManifest
	service          *serviceInfo
	endpoints        map[string]*optionsv1alpha1.Endpoint
	location         descriptor.Location
}

type serviceInfo struct {
	descriptor          protoreflect.ServiceDescriptor
	options             *optionsv1alpha1.ServiceOptionsSpec
	manifest            *manifestv1alpha1.ServiceManifest
	methodMembers       map[protoreflect.FullName]*manifestv1alpha1.ServiceMemberManifest
	eventGroups         map[string]uint32
	payloads            map[protoreflect.FullName]protoreflect.MessageDescriptor
	memberPayloadBounds map[uint32][]memberDirectionalBound
	maxPayload          uint64
}

// memberDirectionalBound preserves one independently bounded member flow.
// It prevents request, response, exception, and notification analysis from
// collapsing into a single aggregate maximum before transport policy is known.
type memberDirectionalBound struct {
	direction manifestv1alpha1.MemberDirection
	size      encodedSize
}

type diagnosticCollector struct {
	diagnostics []*manifestv1alpha1.Diagnostic
}

func (c *diagnosticCollector) add(
	rule rule,
	severity manifestv1alpha1.DiagnosticSeverity,
	location descriptor.Location,
	message string,
) {
	c.diagnostics = append(c.diagnostics, &manifestv1alpha1.Diagnostic{
		RuleCode:           rule.code,
		Severity:           severity,
		Authority:          rule.authority,
		AuthorityReference: rule.reference,
		Location:           location.Manifest(),
		Remediation:        rule.remediation,
		Message:            message,
	})
}

func (c *diagnosticCollector) error(rule rule, location descriptor.Location, message string) {
	c.add(rule, manifestv1alpha1.DiagnosticSeverity_DIAGNOSTIC_SEVERITY_ERROR, location, message)
}

func (c *diagnosticCollector) warning(rule rule, location descriptor.Location, message string) {
	c.add(rule, manifestv1alpha1.DiagnosticSeverity_DIAGNOSTIC_SEVERITY_WARNING, location, message)
}

func (c *diagnosticCollector) sorted() []*manifestv1alpha1.Diagnostic {
	diagnostics := append([]*manifestv1alpha1.Diagnostic(nil), c.diagnostics...)
	sort.Slice(diagnostics, func(i, j int) bool {
		left, right := diagnostics[i], diagnostics[j]
		if left.GetRuleCode() != right.GetRuleCode() {
			return left.GetRuleCode() < right.GetRuleCode()
		}
		if left.GetSeverity() != right.GetSeverity() {
			return left.GetSeverity() < right.GetSeverity()
		}
		if left.GetLocation().GetImportPath() != right.GetLocation().GetImportPath() {
			return left.GetLocation().GetImportPath() < right.GetLocation().GetImportPath()
		}
		if left.GetLocation().GetLine() != right.GetLocation().GetLine() {
			return left.GetLocation().GetLine() < right.GetLocation().GetLine()
		}
		if left.GetLocation().GetColumn() != right.GetLocation().GetColumn() {
			return left.GetLocation().GetColumn() < right.GetLocation().GetColumn()
		}
		return left.GetMessage() < right.GetMessage()
	})
	return diagnostics
}
