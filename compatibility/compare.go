// Package compatibility compares normalized static SOME/IP contracts.
package compatibility

import (
	"sort"
	"strings"

	manifestv1alpha1 "github.com/eshenhu/someip_proto/gen/go/io/github/someip_proto/manifest/v1alpha1"
	optionsv1alpha1 "github.com/eshenhu/someip_proto/gen/go/io/github/someip_proto/options/v1alpha1"
	"github.com/eshenhu/someip_proto/internal/descriptor"
	"github.com/eshenhu/someip_proto/internal/semantic"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Result contains the independently normalized baseline/current results and
// the combined semantic and compatibility diagnostics.
type Result struct {
	Baseline    *semantic.Result
	Current     *semantic.Result
	Diagnostics []*manifestv1alpha1.Diagnostic
}

// HasErrors reports whether normalization or compatibility comparison failed.
func (r *Result) HasErrors() bool {
	for _, diagnostic := range r.Diagnostics {
		if diagnostic.GetSeverity() == manifestv1alpha1.DiagnosticSeverity_DIAGNOSTIC_SEVERITY_ERROR {
			return true
		}
	}
	return false
}

// ReportManifest returns a current-manifest view carrying all comparison
// diagnostics. It returns nil when current normalization did not produce one.
func (r *Result) ReportManifest() *manifestv1alpha1.Manifest {
	if r == nil || r.Current == nil || r.Current.Manifest == nil {
		return nil
	}
	report := protoCloneManifest(r.Current.Manifest)
	report.Diagnostics = append([]*manifestv1alpha1.Diagnostic(nil), r.Diagnostics...)
	return report
}

// Compare normalizes baseline and current under identical tool rules, then
// compares their canonical manifests for the selected profile.
func Compare(
	baseline *descriptor.Set,
	current *descriptor.Set,
	profileName string,
	config semantic.Config,
) *Result {
	if baseline == nil || current == nil {
		return &Result{Diagnostics: []*manifestv1alpha1.Diagnostic{
			compatibilityDiagnostic(nil, "both baseline and current descriptor sets are required for comparison"),
		}}
	}
	baselineResult := semantic.Normalize(baseline, profileName, config)
	currentResult := semantic.Normalize(current, profileName, config)
	result := &Result{
		Baseline: baselineResult,
		Current:  currentResult,
	}
	result.Diagnostics = append(result.Diagnostics, baselineResult.Diagnostics...)
	result.Diagnostics = append(result.Diagnostics, currentResult.Diagnostics...)
	if !baselineResult.HasErrors() && !currentResult.HasErrors() {
		result.Diagnostics = append(result.Diagnostics, CompareManifests(baselineResult.Manifest, currentResult.Manifest)...)
		result.Diagnostics = append(result.Diagnostics, compareMethodContracts(baseline, current, baselineResult.Manifest, currentResult.Manifest)...)
		if !metadataComparisonReliable(baseline, current, baselineResult.Manifest, currentResult.Manifest) {
			result.Diagnostics = append(result.Diagnostics, compatibilityDiagnostic(
				sourceLocation(currentResult.Manifest.GetProfile()),
				"source metadata is incomplete for a stable declaration; cannot verify patch-level comment compatibility",
			))
		} else if metadataPatchChanged(baseline, current, baselineResult.Manifest, currentResult.Manifest) {
			if diagnostic := metadataPatchTransitionDiagnostic(baselineResult.Manifest, currentResult.Manifest); diagnostic != nil {
				result.Diagnostics = append(result.Diagnostics, diagnostic)
			}
		}
	}
	result.Diagnostics = sortDiagnostics(result.Diagnostics)
	return result
}

func metadataPatchTransitionDiagnostic(baseline, current *manifestv1alpha1.Manifest) *manifestv1alpha1.Diagnostic {
	baselineVersion := baseline.GetProfile().GetVersion()
	currentVersion := current.GetProfile().GetVersion()
	if compareVersion(currentVersion, baselineVersion) > 0 {
		return nil
	}
	return compatibilityDiagnostic(
		sourceLocation(current.GetProfile()),
		"documentation, URI, or deprecation metadata changed without a profile patch transition from %s",
		versionText(baselineVersion),
	)
}

func compareMethodContracts(
	baselineSet, currentSet *descriptor.Set,
	baseline, current *manifestv1alpha1.Manifest,
) []*manifestv1alpha1.Diagnostic {
	baselineServices := servicesByKey(baseline.GetServices())
	currentServices := servicesByKey(current.GetServices())
	var diagnostics []*manifestv1alpha1.Diagnostic
	for key, baselineService := range baselineServices {
		currentService, found := currentServices[key]
		if !found || baselineService.GetService().GetFqn() != currentService.GetService().GetFqn() {
			continue
		}
		baselineMethods, ok := methodsByMemberID(baselineSet, baselineService)
		if !ok {
			continue
		}
		currentMethods, ok := methodsByMemberID(currentSet, currentService)
		if !ok {
			continue
		}
		for memberID, baselineMethod := range baselineMethods {
			currentMethod, found := currentMethods[memberID]
			if !found || (baselineMethod.descriptor.FullName() == currentMethod.descriptor.FullName() && sameMethodContract(baselineMethod.options, currentMethod.options)) {
				continue
			}
			diagnostics = append(diagnostics, compatibilityDiagnostic(
				currentSet.Locations.Descriptor(currentMethod.descriptor).Manifest(),
				"method %s changes return-code, response-policy, exception-payload, or timing behavior without an interface-major transition",
				currentMethod.descriptor.FullName(),
			))
		}
	}
	return diagnostics
}

type methodContract struct {
	descriptor protoreflect.MethodDescriptor
	options    *optionsv1alpha1.MethodOptionsSpec
}

func methodsByMemberID(set *descriptor.Set, service *manifestv1alpha1.ServiceManifest) (map[uint32]methodContract, bool) {
	name := strings.TrimPrefix(service.GetService().GetFqn(), ".")
	descriptorValue, err := set.Files.FindDescriptorByName(protoreflect.FullName(name))
	if err != nil {
		return nil, false
	}
	descriptorService, ok := descriptorValue.(protoreflect.ServiceDescriptor)
	if !ok {
		return nil, false
	}
	methods := make(map[uint32]methodContract, descriptorService.Methods().Len())
	for index := range descriptorService.Methods().Len() {
		method := descriptorService.Methods().Get(index)
		options := descriptor.MethodOptions(method)
		if options == nil {
			return nil, false
		}
		methods[options.GetMemberId()] = methodContract{descriptor: method, options: options}
	}
	return methods, true
}

func sameMethodContract(left, right *optionsv1alpha1.MethodOptionsSpec) bool {
	if left == nil || right == nil {
		return left == right
	}
	if left.GetKind() != right.GetKind() ||
		left.GetResponsePolicy() != right.GetResponsePolicy() ||
		left.GetExceptionPayloadType().GetFqn() != right.GetExceptionPayloadType().GetFqn() ||
		!sameMethodTiming(left.GetTiming(), right.GetTiming()) {
		return false
	}
	return sameReturnCodes(left.GetReturnCodes(), right.GetReturnCodes())
}

func sameMethodTiming(left, right *optionsv1alpha1.MethodTiming) bool {
	if left == nil && right == nil {
		return true
	}
	if left == nil {
		return right.GetResponseTimeout() == nil && right.GetRetryDelay() == nil
	}
	if right == nil {
		return left.GetResponseTimeout() == nil && left.GetRetryDelay() == nil
	}
	return proto.Equal(left.GetResponseTimeout(), right.GetResponseTimeout()) &&
		proto.Equal(left.GetRetryDelay(), right.GetRetryDelay())
}

func sameReturnCodes(left, right []*optionsv1alpha1.ReturnCode) bool {
	if len(left) != len(right) {
		return false
	}
	byCode := make(map[uint32]string, len(left))
	for _, code := range left {
		byCode[code.GetCode()] = code.GetName()
	}
	for _, code := range right {
		if byCode[code.GetCode()] != code.GetName() {
			return false
		}
	}
	return true
}

func protoCloneManifest(manifest *manifestv1alpha1.Manifest) *manifestv1alpha1.Manifest {
	if manifest == nil {
		return nil
	}
	return proto.Clone(manifest).(*manifestv1alpha1.Manifest)
}

func sortDiagnostics(diagnostics []*manifestv1alpha1.Diagnostic) []*manifestv1alpha1.Diagnostic {
	ordered := append([]*manifestv1alpha1.Diagnostic(nil), diagnostics...)
	sort.Slice(ordered, func(i, j int) bool {
		left, right := ordered[i], ordered[j]
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
	return ordered
}
