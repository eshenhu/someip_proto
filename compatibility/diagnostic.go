package compatibility

import (
	"fmt"

	manifestv1alpha1 "github.com/eshenhu/someip_proto/gen/go/io/github/someip_proto/manifest/v1alpha1"
)

type changeLevel uint8

const (
	changeNone changeLevel = iota
	changePatch
	changeMinor
	changeMajor
)

func (l changeLevel) String() string {
	switch l {
	case changePatch:
		return "patch"
	case changeMinor:
		return "minor"
	case changeMajor:
		return "major"
	default:
		return "none"
	}
}

func maximumChange(left, right changeLevel) changeLevel {
	if left > right {
		return left
	}
	return right
}

func compatibilityDiagnostic(location *manifestv1alpha1.SourceLocation, format string, arguments ...any) *manifestv1alpha1.Diagnostic {
	return &manifestv1alpha1.Diagnostic{
		RuleCode:           "SIP-COMPAT-001",
		Severity:           manifestv1alpha1.DiagnosticSeverity_DIAGNOSTIC_SEVERITY_ERROR,
		Authority:          manifestv1alpha1.RuleAuthority_RULE_AUTHORITY_NORMATIVE,
		AuthorityReference: "src/someip-compat.rst#feat_req_someipcompat_714",
		Location:           location,
		Remediation:        "Increase the required interface or profile semantic-version component before publishing the resolved contract change.",
		Message:            fmt.Sprintf(format, arguments...),
	}
}

func staticEvolutionDiagnostic(location *manifestv1alpha1.SourceLocation, format string, arguments ...any) *manifestv1alpha1.Diagnostic {
	return &manifestv1alpha1.Diagnostic{
		RuleCode:           "SIP-COMPAT-002",
		Severity:           manifestv1alpha1.DiagnosticSeverity_DIAGNOSTIC_SEVERITY_ERROR,
		Authority:          manifestv1alpha1.RuleAuthority_RULE_AUTHORITY_PROJECT_POLICY,
		AuthorityReference: "docs/adr/0004-static-authoring-policy.md",
		Location:           location,
		Remediation:        "Keep fixed layouts immutable; append only higher wire-order elements to extensible length-delimited structs.",
		Message:            fmt.Sprintf(format, arguments...),
	}
}
