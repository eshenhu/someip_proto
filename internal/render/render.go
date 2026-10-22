// Package render produces deterministic views of normalized SOME/IP manifests.
package render

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	manifestv1alpha1 "github.com/eshenhu/someip_proto/gen/go/io/github/someip_proto/manifest/v1alpha1"
	optionsv1alpha1 "github.com/eshenhu/someip_proto/gen/go/io/github/someip_proto/options/v1alpha1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// Binary returns the deterministic protobuf binary representation of manifest.
func Binary(manifest *manifestv1alpha1.Manifest) ([]byte, error) {
	if manifest == nil {
		return nil, fmt.Errorf("render binary manifest: manifest is nil")
	}
	return proto.MarshalOptions{Deterministic: true}.Marshal(manifest)
}

// JSON returns canonical protobuf JSON for manifest with a trailing newline.
func JSON(manifest *manifestv1alpha1.Manifest) ([]byte, error) {
	if manifest == nil {
		return nil, fmt.Errorf("render JSON manifest: manifest is nil")
	}
	encoded, err := (protojson.MarshalOptions{
		UseProtoNames: true,
		Indent:        "  ",
	}).Marshal(manifest)
	if err != nil {
		return nil, fmt.Errorf("render JSON manifest: %w", err)
	}
	return canonicalJSON(encoded)
}

// Markdown returns a deterministic human-readable view of manifest.
func Markdown(manifest *manifestv1alpha1.Manifest) ([]byte, error) {
	if manifest == nil {
		return nil, fmt.Errorf("render Markdown manifest: manifest is nil")
	}
	var output strings.Builder
	profile := manifest.GetProfile()
	profileName := "(none)"
	if profile != nil && profile.GetProfileName() != "" {
		profileName = profile.GetProfileName()
	}
	fmt.Fprintf(&output, "# SOME/IP manifest: %s\n\n", markdownText(profileName))
	renderProvenance(&output, manifest.GetProvenance())
	renderProfile(&output, profile)
	renderServices(&output, manifest.GetServices())
	renderPayloads(&output, manifest.GetPayloadTypes())
	renderEnums(&output, manifest.GetEnumTypes())
	renderUnknownOptions(&output, manifest.GetUnknownOptions())
	renderDiagnostics(&output, manifest.GetDiagnostics())
	return []byte(output.String()), nil
}

// HTML returns a self-contained, escaped static HTML view of manifest. It has
// no executable script, remote assets, or descriptor-derived links.
func HTML(manifest *manifestv1alpha1.Manifest) ([]byte, error) {
	if manifest == nil {
		return nil, fmt.Errorf("render HTML manifest: manifest is nil")
	}
	var output strings.Builder
	profileName := manifest.GetProfile().GetProfileName()
	if profileName == "" {
		profileName = "(none)"
	}
	errors, warnings, infos := diagnosticCounts(manifest.GetDiagnostics())
	fmt.Fprintf(&output, `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>SOME/IP manifest: %s</title>
<style>%s</style>
</head>
<body><main>
<header class="hero"><p class="eyebrow">SOME/IP contract report</p><h1>%s</h1><p class="muted">Static manifest overview and validation findings</p></header>
<section class="summary-grid" aria-label="Diagnostic summary">
<div class="summary-card error"><strong>%d</strong><span>Errors</span></div>
<div class="summary-card warning"><strong>%d</strong><span>Warnings</span></div>
<div class="summary-card info"><strong>%d</strong><span>Informational</span></div>
<div class="summary-card neutral"><strong>%d</strong><span>Total findings</span></div>
</section>
`, htmlText(profileName), htmlReportCSS, htmlText(profileName), errors, warnings, infos, len(manifest.GetDiagnostics()))
	renderHTMLProvenance(&output, manifest.GetProvenance())
	renderHTMLProfile(&output, manifest.GetProfile())
	renderHTMLServices(&output, manifest.GetServices())
	renderHTMLPayloads(&output, manifest.GetPayloadTypes())
	renderHTMLEnums(&output, manifest.GetEnumTypes())
	renderHTMLUnknownOptions(&output, manifest.GetUnknownOptions())
	renderHTMLDiagnostics(&output, manifest.GetDiagnostics())
	output.WriteString("</main></body></html>\n")
	return []byte(output.String()), nil
}

// TextDiagnostics returns stable, line-oriented diagnostics suitable for a
// terminal or plain-text report.
func TextDiagnostics(diagnostics []*manifestv1alpha1.Diagnostic) []byte {
	ordered := sortedDiagnostics(diagnostics)
	var output strings.Builder
	for _, diagnostic := range ordered {
		location := diagnosticLocation(diagnostic.GetLocation())
		fmt.Fprintf(
			&output,
			"%s %s %s: %s [%s %s; remediation: %s]\n",
			diagnostic.GetSeverity().String(),
			diagnostic.GetRuleCode(),
			location,
			diagnostic.GetMessage(),
			diagnostic.GetAuthority().String(),
			diagnostic.GetAuthorityReference(),
			diagnostic.GetRemediation(),
		)
	}
	return []byte(output.String())
}

// JSONDiagnostics returns canonical protobuf JSON containing diagnostics in
// their stable display order.
func JSONDiagnostics(diagnostics []*manifestv1alpha1.Diagnostic) ([]byte, error) {
	report := &manifestv1alpha1.DiagnosticReport{
		DiagnosticFormatVersion: 1,
		Diagnostics:             sortedDiagnostics(diagnostics),
	}
	encoded, err := (protojson.MarshalOptions{
		UseProtoNames: true,
		Indent:        "  ",
	}).Marshal(report)
	if err != nil {
		return nil, fmt.Errorf("render JSON diagnostics: %w", err)
	}
	return canonicalJSON(encoded)
}

func canonicalJSON(encoded []byte) ([]byte, error) {
	var formatted bytes.Buffer
	if err := json.Indent(&formatted, encoded, "", "  "); err != nil {
		return nil, fmt.Errorf("indent canonical JSON: %w", err)
	}
	return append(formatted.Bytes(), '\n'), nil
}

func renderProvenance(output *strings.Builder, provenance *manifestv1alpha1.ManifestProvenance) {
	fmt.Fprintln(output, "## Provenance")
	if provenance == nil {
		fmt.Fprintln(output, "")
		fmt.Fprintln(output, "No provenance was recorded.")
		fmt.Fprintln(output, "")
		return
	}
	fmt.Fprintf(output, "- Manifest format: `%d`\n", provenance.GetManifestFormatVersion())
	fmt.Fprintf(output, "- Options API: `%s`\n", markdownText(provenance.GetOptionsApiVersion()))
	fmt.Fprintf(output, "- Input FDS SHA-256: `%s`\n", markdownText(provenance.GetInputFdsSha256()))
	fmt.Fprintf(output, "- Tool version: `%s`\n", markdownText(provenance.GetToolVersion()))
	if provenance.GetSourceRevision() != "" {
		fmt.Fprintf(output, "- Source revision: `%s`\n", markdownText(provenance.GetSourceRevision()))
	}
	fmt.Fprintf(output, "- Normative source commit: `%s`\n\n", markdownText(provenance.GetNormativeSourceCommit()))
}

func renderProfile(output *strings.Builder, profile *manifestv1alpha1.ProfileManifest) {
	fmt.Fprintln(output, "## Deployment profile")
	if profile == nil {
		fmt.Fprintln(output, "")
		fmt.Fprintln(output, "No deployment profile was selected.")
		fmt.Fprintln(output, "")
		return
	}
	fmt.Fprintf(output, "- Name: `%s`\n", markdownText(profile.GetProfileName()))
	fmt.Fprintf(output, "- Version: `%s`\n", markdownText(semanticVersion(profile.GetVersion())))
	fmt.Fprintf(output, "- UDP application payload budget: `%d` bytes\n", profile.GetUdpApplicationPayloadBudgetBytes())
	renderDocumentationURIs(output, "", profile.GetDocumentationUris())
	fmt.Fprintln(output, "")

	fmt.Fprintln(output, "### Provider identities")
	if len(profile.GetProviderInstances()) == 0 {
		fmt.Fprintln(output, "")
		fmt.Fprintln(output, "None.")
		fmt.Fprintln(output, "")
	} else {
		for _, provider := range profile.GetProviderInstances() {
			instance := provider.GetInstance()
			fmt.Fprintf(output, "#### `%s`\n\n", markdownText(instance.GetProviderName()))
			fmt.Fprintf(output, "- Service/instance ID: `%d` / `%d`\n", instance.GetServiceId(), instance.GetInstanceId())
			fmt.Fprintf(output, "- Lineage: `%s`\n", markdownText(instance.GetServiceLineage().GetId()))
			fmt.Fprintf(output, "- Default endpoint: `%s`\n", markdownText(instance.GetDefaultEndpointName()))
			if len(instance.GetEndpoints()) > 0 {
				fmt.Fprintln(output, "- Endpoints:")
				for _, endpoint := range instance.GetEndpoints() {
					fmt.Fprintf(output, "  - `%s`: `%s` `%s` port `%d`\n", markdownText(endpoint.GetName()), markdownText(endpoint.GetTransport().String()), markdownText(endpoint.GetAddress()), endpoint.GetPort())
				}
			}
			fmt.Fprintf(output, "- Maximum application payload: `%d` bytes\n", provider.GetMaximumApplicationPayloadBytes())
			fmt.Fprintf(output, "- Maximum SOME/IP message: `%d` bytes\n", provider.GetMaximumSomeipMessageBytes())
			fmt.Fprintln(output, "- Service-version bindings:")
			for _, binding := range provider.GetServiceVersionBindings() {
				fmt.Fprintf(
					output,
					"  - `%s` (lineage `%s`, major `%d`, maximum payload `%d` bytes)\n",
					markdownText(binding.GetBinding().GetService().GetFqn()),
					markdownText(binding.GetServiceLineage().GetId()),
					binding.GetBinding().GetInterfaceMajorVersion(),
					binding.GetMaximumApplicationPayloadBytes(),
				)
				for _, override := range binding.GetBinding().GetMemberEndpointOverrides() {
					fmt.Fprintf(output, "    - Member `%d` endpoint override: `%s`\n", override.GetMemberId(), markdownText(override.GetEndpointName()))
				}
				renderProviderStaticContract(output, binding, "    ")
			}
			fmt.Fprintln(output, "")
		}
	}
	renderConsumerBindings(output, profile.GetConsumerBindings())
}

func renderProviderStaticContract(output *strings.Builder, binding *manifestv1alpha1.ProviderServiceVersionBindingManifest, indent string) {
	if binding == nil {
		return
	}
	offer := binding.GetServiceDiscoveryOffer()
	if offer == nil {
		fmt.Fprintf(output, "%s- Static Service Discovery offer: none\n", indent)
	} else {
		fmt.Fprintf(output, "%s- Static Service Discovery offer:\n", indent)
		if endpoints := offer.GetEndpointNames(); len(endpoints) == 0 {
			fmt.Fprintf(output, "%s  - Advertised endpoints: none\n", indent)
		} else {
			fmt.Fprintf(output, "%s  - Advertised endpoints: %s\n", indent, markdownStringList(endpoints))
		}
		renderServiceDiscoveryTiming(output, offer.GetTiming(), indent+"  ")
	}
	policies := binding.GetEventGroupDeliveryPolicies()
	if len(policies) == 0 && offer != nil {
		policies = offer.GetEventGroupDeliveryPolicies()
	}
	if len(policies) == 0 {
		fmt.Fprintf(output, "%s- Event-group delivery policies: none\n", indent)
	} else {
		fmt.Fprintf(output, "%s- Event-group delivery policies:\n", indent)
		for _, policy := range policies {
			fmt.Fprintf(output, "%s  - `%s`: `%s`%s\n", indent, markdownText(policy.GetEventGroupName()), markdownText(deliveryModeText(policy.GetMode())), markdownText(multicastEndpointSuffix(policy.GetMulticastEndpoint())))
		}
	}
	if configurations := binding.GetMemberTpConfigurations(); len(configurations) == 0 {
		fmt.Fprintf(output, "%s- Provider TP declarations: none\n", indent)
	} else {
		fmt.Fprintf(output, "%s- Provider TP declarations:\n", indent)
		for _, configuration := range configurations {
			fmt.Fprintf(output, "%s  - Member `%d`: `tp_when_needed=%t`\n", indent, configuration.GetMemberId(), configuration.GetTpWhenNeeded())
		}
	}
	renderTPAnalyses(output, binding.GetMemberTpAnalysis(), indent, "Derived provider TP analysis")
}

func renderConsumerBindings(output *strings.Builder, consumers []*manifestv1alpha1.ConsumerServiceBindingManifest) {
	fmt.Fprintln(output, "### Consumer bindings")
	if len(consumers) == 0 {
		fmt.Fprintln(output, "")
		fmt.Fprintln(output, "None.")
		fmt.Fprintln(output, "")
		return
	}
	for _, consumer := range consumers {
		binding := consumer.GetBinding()
		fmt.Fprintf(output, "#### `%s`\n\n", markdownText(binding.GetConsumerName()))
		fmt.Fprintf(output, "- Target service: `%s` (lineage `%s`, major `%d`)\n", markdownText(binding.GetService().GetFqn()), markdownText(binding.GetServiceLineage().GetId()), binding.GetInterfaceMajorVersion())
		fmt.Fprintf(output, "- Default local endpoint: `%s`\n", markdownText(binding.GetDefaultEndpointName()))
		renderProviderSelector(output, consumer.GetProviderSelector(), "")
		if endpoints := consumer.GetLocalEndpoints(); len(endpoints) == 0 {
			fmt.Fprintln(output, "- Local endpoints: none")
		} else {
			fmt.Fprintln(output, "- Local endpoints:")
			for _, endpoint := range endpoints {
				fmt.Fprintf(output, "  - `%s`: `%s` `%s` port `%s`\n", markdownText(endpoint.GetName()), markdownText(endpoint.GetTransport().String()), markdownText(endpoint.GetAddress()), markdownText(consumerEndpointPortText(endpoint)))
			}
		}
		if overrides := binding.GetMemberEndpointOverrides(); len(overrides) > 0 {
			fmt.Fprintln(output, "- Member local-endpoint overrides:")
			for _, override := range overrides {
				fmt.Fprintf(output, "  - Member `%d`: `%s`\n", override.GetMemberId(), markdownText(override.GetEndpointName()))
			}
		}
		renderServiceDiscoveryTiming(output, consumer.GetFindTiming(), "")
		renderConsumerSubscriptions(output, consumer.GetSubscriptions())
		renderTPAnalyses(output, consumer.GetInheritedMemberTpAnalysis(), "", "Inherited provider TP analysis")
		fmt.Fprintln(output, "")
	}
}

func renderProviderSelector(output *strings.Builder, selector *manifestv1alpha1.ResolvedProviderSelectorManifest, indent string) {
	if selector == nil || selector.GetSelector() == nil {
		fmt.Fprintf(output, "%s- Provider selector: none\n", indent)
		return
	}
	authored := selector.GetSelector()
	selection := ""
	switch value := authored.GetSelector().(type) {
	case *optionsv1alpha1.ProviderSelector_NamedProviderName:
		selection = fmt.Sprintf("named provider `%s`", markdownText(value.NamedProviderName))
	case *optionsv1alpha1.ProviderSelector_AnyMatchingInstance:
		selection = fmt.Sprintf("any matching instance (`%t`, `%s`)", value.AnyMatchingInstance, markdownText(providerSelectionModeText(authored.GetSelectionMode())))
	default:
		selection = "unspecified"
	}
	fmt.Fprintf(output, "%s- Provider selector: %s\n", indent, selection)
	candidates := selector.GetCandidates()
	if len(candidates) == 0 {
		fmt.Fprintf(output, "%s  - Candidates: none\n", indent)
		return
	}
	fmt.Fprintf(output, "%s  - Candidates:\n", indent)
	for _, candidate := range candidates {
		fmt.Fprintf(output, "%s    - `%s` (service/instance `%d` / `%d`, `%s` major `%d`, lineage `%s`)\n", indent, markdownText(candidate.GetProviderName()), candidate.GetServiceId(), candidate.GetInstanceId(), markdownText(candidate.GetService().GetFqn()), candidate.GetInterfaceMajorVersion(), markdownText(candidate.GetServiceLineage().GetId()))
	}
}

func renderServiceDiscoveryTiming(output *strings.Builder, timing *optionsv1alpha1.ServiceDiscoveryTiming, indent string) {
	if timing == nil {
		fmt.Fprintf(output, "%s- Service Discovery timing: none\n", indent)
		return
	}
	fmt.Fprintf(output, "%s- Service Discovery timing: live TTL `%d` seconds\n", indent, timing.GetLiveTtlSeconds())
	if timing.GetInitialDelay() == nil && timing.GetRepetitionsBaseDelay() == nil && timing.GetRepetitionsMax() == 0 && timing.GetRequestResponseDelay() == nil && timing.GetCyclicOfferDelay() == nil {
		fmt.Fprintf(output, "%s  - TTL-only timing\n", indent)
		return
	}
	if initial := timing.GetInitialDelay(); initial != nil {
		fmt.Fprintf(output, "%s  - Initial delay: `%s`\n", indent, durationRangeText(initial))
	}
	if delay := timing.GetRepetitionsBaseDelay(); delay != nil {
		fmt.Fprintf(output, "%s  - Repetitions: `%d` every `%s`\n", indent, timing.GetRepetitionsMax(), durationText(delay.GetSeconds(), delay.GetNanos()))
	}
	if delay := timing.GetRequestResponseDelay(); delay != nil {
		fmt.Fprintf(output, "%s  - Request-response delay: `%s`\n", indent, durationRangeText(delay))
	}
	if delay := timing.GetCyclicOfferDelay(); delay != nil {
		fmt.Fprintf(output, "%s  - Cyclic delay: `%s`\n", indent, durationText(delay.GetSeconds(), delay.GetNanos()))
	}
}

func renderConsumerSubscriptions(output *strings.Builder, subscriptions []*manifestv1alpha1.ConsumerSubscriptionManifest) {
	if len(subscriptions) == 0 {
		fmt.Fprintln(output, "- Subscriptions: none")
		return
	}
	fmt.Fprintln(output, "- Subscriptions:")
	for _, subscription := range subscriptions {
		authored := subscription.GetSubscription()
		fmt.Fprintf(output, "  - Event group `%s` (ID `%d`, TTL `%d` seconds, initial data `%t`)\n", markdownText(authored.GetEventGroupName()), subscription.GetEventGroupId(), authored.GetTtlSeconds(), subscription.GetInitialDataRequested())
		if names := authored.GetLocalEndpointNames(); len(names) == 0 {
			fmt.Fprintln(output, "    - Local endpoints: none")
		} else {
			fmt.Fprintf(output, "    - Local endpoints: %s\n", markdownStringList(names))
		}
		if policy := subscription.GetDeliveryPolicy(); policy == nil {
			fmt.Fprintln(output, "    - Effective delivery policy: provider-specific or unavailable")
		} else {
			fmt.Fprintf(output, "    - Effective delivery policy: `%s`%s\n", markdownText(deliveryModeText(policy.GetMode())), markdownText(multicastEndpointSuffix(policy.GetMulticastEndpoint())))
		}
	}
}

func renderTPAnalyses(output *strings.Builder, analyses []*manifestv1alpha1.ProviderMemberTpAnalysisManifest, indent, title string) {
	if len(analyses) == 0 {
		fmt.Fprintf(output, "%s- %s: none\n", indent, title)
		return
	}
	fmt.Fprintf(output, "%s- %s:\n", indent, title)
	for _, analysis := range analyses {
		fmt.Fprintf(output, "%s  - Member `%d` (`tp_when_needed=%t`):\n", indent, analysis.GetMemberId(), analysis.GetTpWhenNeeded())
		for _, direction := range analysis.GetDirections() {
			endpoint := effectiveEndpointText(direction.GetEndpoint())
			if endpoint == "" {
				endpoint = "(no candidate-specific endpoint)"
			}
			fmt.Fprintf(output, "%s    - `%s`: payload %s; transport `%s`; endpoint `%s`; result `%s`; UDP budget `%d` bytes; runtime TP/session handling `%t`\n", indent, markdownText(memberDirectionText(direction.GetDirection())), encodedSizeText(direction.GetEncodedSize()), markdownText(effectiveTransportText(direction.GetEffectiveTransport())), markdownText(endpoint), markdownText(tpUseResultText(direction.GetTpUseResult())), direction.GetUnsegmentedUdpPayloadBudgetBytes(), direction.GetRuntimeTpSessionHandlingRequired())
		}
	}
}

func renderServices(output *strings.Builder, services []*manifestv1alpha1.ServiceManifest) {
	fmt.Fprintln(output, "## Services")
	if len(services) == 0 {
		fmt.Fprintln(output, "")
		fmt.Fprintln(output, "None.")
		fmt.Fprintln(output, "")
		return
	}
	for _, service := range services {
		fmt.Fprintf(output, "### `%s`\n\n", markdownText(service.GetService().GetFqn()))
		fmt.Fprintf(output, "- Lineage: `%s`\n", markdownText(service.GetServiceLineage().GetId()))
		fmt.Fprintf(output, "- Interface version: `%d.%d`\n", service.GetInterfaceMajorVersion(), service.GetInterfaceMinorVersion())
		renderDocumentationURIs(output, "", service.GetDocumentationUris())
		if len(service.GetEventGroups()) > 0 {
			fmt.Fprintln(output, "- Event groups:")
			for _, group := range service.GetEventGroups() {
				fmt.Fprintf(output, "  - `%d` `%s`\n", group.GetId(), markdownText(group.GetName()))
			}
		}
		if len(service.GetMembers()) == 0 {
			fmt.Fprintln(output, "- Members: none")
		} else {
			fmt.Fprintln(output, "- Members:")
			for _, member := range service.GetMembers() {
				fmt.Fprintf(output, "  - `%d` `%s` (`%s`)\n", member.GetMemberId(), markdownText(member.GetName()), markdownText(member.GetKind()))
				if method := member.GetMethodOptions(); method != nil {
					fmt.Fprintf(output, "    - Method kind: `%s`; result policy: `%s`\n", method.GetKind().String(), method.GetResponsePolicy().String())
					renderDocumentationURIs(output, "    ", method.GetDocumentationUris())
					if exception := method.GetExceptionPayloadType().GetFqn(); exception != "" {
						fmt.Fprintf(output, "    - Exception payload: `%s`\n", markdownText(exception))
					}
					if len(method.GetReturnCodes()) > 0 {
						fmt.Fprintln(output, "    - Return codes:")
						for _, code := range method.GetReturnCodes() {
							fmt.Fprintf(output, "      - `%d` `%s`%s\n", code.GetCode(), markdownText(code.GetName()), optionalDescription(code.GetDescription()))
						}
					}
					renderMethodTiming(output, method)
				}
				if len(member.GetEventGroupNames()) > 0 {
					fmt.Fprintf(output, "    - Event groups: %s\n", markdownStringList(member.GetEventGroupNames()))
				}
				if publication := member.GetPublication(); publication != nil {
					renderPublication(output, publication, "    ")
				}
				if directionalPayloads := member.GetDirectionalPayloads(); len(directionalPayloads) > 0 {
					fmt.Fprintln(output, "    - Directional payload bounds:")
					for _, payload := range directionalPayloads {
						fmt.Fprintf(output, "      - `%s`: %s\n", markdownText(memberDirectionText(payload.GetDirection())), encodedSizeText(payload.GetEncodedSize()))
					}
				}
			}
		}
		if len(service.GetFields()) > 0 {
			fmt.Fprintln(output, "- Logical fields:")
			for _, normalizedField := range service.GetFields() {
				field := normalizedField.GetField()
				fmt.Fprintf(output, "  - `%s` value `%s`\n", markdownText(field.GetName()), markdownText(field.GetValueType().GetFqn()))
				if getter := field.GetGetterMethod().GetFqn(); getter != "" {
					fmt.Fprintf(output, "    - Getter: `%s`\n", markdownText(getter))
				}
				if setter := field.GetSetterMethod().GetFqn(); setter != "" {
					fmt.Fprintf(output, "    - Setter: `%s`\n", markdownText(setter))
				}
				if field.GetNotifierId() != 0 {
					fmt.Fprintf(output, "    - Notifier ID: `%d`\n", field.GetNotifierId())
				}
				if len(field.GetEventGroupNames()) > 0 {
					fmt.Fprintf(output, "    - Event groups: %s\n", markdownStringList(field.GetEventGroupNames()))
				}
				if publication := field.GetPublication(); publication != nil {
					renderPublication(output, publication, "    ")
				}
			}
		}
		fmt.Fprintln(output, "")
	}
}

func renderPayloads(output *strings.Builder, payloads []*manifestv1alpha1.PayloadTypeManifest) {
	fmt.Fprintln(output, "## Payload types")
	if len(payloads) == 0 {
		fmt.Fprintln(output, "")
		fmt.Fprintln(output, "None.")
		fmt.Fprintln(output, "")
		return
	}
	for _, payload := range payloads {
		fmt.Fprintf(output, "### `%s`\n\n", markdownText(payload.GetType().GetFqn()))
		fmt.Fprintf(output, "- Encoded size: %s\n", encodedSizeText(payload.GetEncodedSize()))
		layout := payload.GetEffectiveLayout()
		if layout != nil {
			fmt.Fprintf(output, "- Byte order: `%s`\n", layout.GetByteOrder().String())
			fmt.Fprintf(output, "- Struct length width: `%s`\n", layout.GetLengthFieldWidth().String())
			fmt.Fprintf(output, "- Extensible: `%t`\n", layout.GetExtensible())
		}
		renderDocumentationURIs(output, "", payload.GetOptions().GetDocumentationUris())
		fmt.Fprintln(output, "- Layout:")
		for _, element := range payload.GetLayoutElements() {
			switch {
			case element.GetMember() != nil:
				member := element.GetMember()
				fmt.Fprintf(output, "  - Member `%s` (wire order `%d`, field `%d`): %s\n", markdownText(member.GetName()), member.GetOptions().GetWireOrder(), member.GetFieldNumber(), encodedSizeText(member.GetEncodedSize()))
				if typeName := member.GetType().GetFqn(); typeName != "" {
					fmt.Fprintf(output, "    - Type: `%s`\n", markdownText(typeName))
				}
				renderDocumentationURIs(output, "    ", member.GetOptions().GetDocumentationUris())
				if effective := member.GetEffectiveLayout(); effective != nil {
					fmt.Fprintf(output, "    - Effective byte order: `%s`; framing width: `%s`\n", effective.GetByteOrder().String(), effective.GetLengthFieldWidth().String())
					if framing := effective.GetStringFraming(); framing != nil {
						fmt.Fprintf(output, "    - String framing: BOM `%d` bytes; terminator `%d` bytes; fixed zero-fill `%t`\n", framing.GetBomBytes(), framing.GetTerminatorBytes(), framing.GetFixedLengthZeroFill())
					}
				}
				if alignment := member.GetAlignment(); alignment != nil {
					fmt.Fprintf(output, "    - Alignment: `%d` bytes; proven `%t`; offsets `%d..%d`\n", alignment.GetRequiredAlignmentBytes(), alignment.GetProven(), alignment.GetMinimumOffsetBytes(), alignment.GetMaximumOffsetBytes())
				}
			case element.GetUnion() != nil:
				union := element.GetUnion()
				fmt.Fprintf(output, "  - Union: %s; effective value slot `%d` bytes\n", encodedSizeText(union.GetEncodedSize()), union.GetEffectiveValueSlotBytes())
				fmt.Fprintf(output, "    - Declared headers: discriminator `%d` bits; length `%s`; order `%s`; declared padding `%d` bytes\n", union.GetOptions().GetDiscriminatorWidthBits(), union.GetOptions().GetLengthFieldWidth().String(), union.GetOptions().GetHeaderOrder().String(), union.GetOptions().GetPaddingBytes())
				renderDocumentationURIs(output, "    ", union.GetOptions().GetDocumentationUris())
				if layout := union.GetEffectiveLayout(); layout != nil {
					fmt.Fprintf(output, "    - Effective header byte order: `%s`; type `%d` bits; length `%s`; NULL allowed `%t`\n", union.GetHeaderByteOrder().String(), layout.GetDiscriminatorWidthBits(), layout.GetLengthFieldWidth().String(), layout.GetAllowsNull())
					fmt.Fprintf(output, "    - Header order: `%s`\n", layout.GetHeaderOrder().String())
				}
				for _, alternative := range union.GetAlternatives() {
					padding := fmt.Sprintf("declared padding `%d`", alternative.GetDeclaredPaddingBytes())
					if alternative.GetPadToEffectiveSlot() {
						padding += ", dynamic zero-fill to effective slot"
					} else if alternative.GetFixedDerivedZeroFillBytes() != 0 {
						padding += fmt.Sprintf(", derived zero-fill `%d`", alternative.GetFixedDerivedZeroFillBytes())
					}
					fmt.Fprintf(output, "    - Type `%d` `%s`: %s (%s)\n", alternative.GetTypeId(), markdownText(alternative.GetName()), encodedSizeText(alternative.GetEncodedSize()), padding)
					renderDocumentationURIs(output, "      ", alternative.GetOptions().GetDocumentationUris())
					if alignment := alternative.GetAlignment(); alignment != nil {
						fmt.Fprintf(output, "      - Alignment: `%d` bytes; proven `%t`; offsets `%d..%d`\n", alignment.GetRequiredAlignmentBytes(), alignment.GetProven(), alignment.GetMinimumOffsetBytes(), alignment.GetMaximumOffsetBytes())
					}
				}
			}
		}
		fmt.Fprintln(output, "")
	}
}

func renderEnums(output *strings.Builder, enums []*manifestv1alpha1.EnumManifest) {
	fmt.Fprintln(output, "## Enum types")
	if len(enums) == 0 {
		fmt.Fprintln(output, "")
		fmt.Fprintln(output, "None.")
		fmt.Fprintln(output, "")
		return
	}
	for _, enum := range enums {
		fmt.Fprintf(output, "### `%s`\n\n", markdownText(enum.GetType().GetFqn()))
		fmt.Fprintf(output, "- Wire width: `%d` bits\n", enum.GetOptions().GetWireWidthBits())
		fmt.Fprintf(output, "- Closed: `%t`\n", enum.GetOptions().GetClosed())
		renderDocumentationURIs(output, "", enum.GetOptions().GetDocumentationUris())
		fmt.Fprintln(output, "- Values:")
		for _, value := range enum.GetValues() {
			placeholder := ""
			if value.GetOptions().GetNonSendablePlaceholder() {
				placeholder = " (non-sendable placeholder)"
			}
			fmt.Fprintf(output, "  - `%d` `%s`%s\n", value.GetNumber(), markdownText(value.GetName()), placeholder)
			renderDocumentationURIs(output, "    ", value.GetOptions().GetDocumentationUris())
		}
		fmt.Fprintln(output, "")
	}
}

func renderUnknownOptions(output *strings.Builder, options []*manifestv1alpha1.UnknownOption) {
	fmt.Fprintln(output, "## Preserved unknown options")
	if len(options) == 0 {
		fmt.Fprintln(output, "")
		fmt.Fprintln(output, "None.")
		fmt.Fprintln(output, "")
		return
	}
	for _, option := range options {
		name := option.GetFullName()
		if name == "" {
			name = "(unresolved)"
		}
		fmt.Fprintf(output, "- `%s` field `%d` on `%s` at %s; wire type `%d`\n", markdownText(name), option.GetFieldNumber(), option.GetExtendeeKind().String(), markdownText(diagnosticLocation(option.GetLocation())), option.GetWireType())
		fmt.Fprintf(output, "  - Raw value (base64): `%s`\n", base64.StdEncoding.EncodeToString(option.GetRawValue()))
		fmt.Fprintf(output, "  - Encoded field (base64): `%s`\n", base64.StdEncoding.EncodeToString(option.GetEncodedField()))
	}
	fmt.Fprintln(output, "")
}

func renderDiagnostics(output *strings.Builder, diagnostics []*manifestv1alpha1.Diagnostic) {
	fmt.Fprintln(output, "## Diagnostics")
	ordered := sortedDiagnostics(diagnostics)
	if len(ordered) == 0 {
		fmt.Fprintln(output, "")
		fmt.Fprintln(output, "None.")
		fmt.Fprintln(output, "")
		return
	}
	for _, diagnostic := range ordered {
		fmt.Fprintf(
			output,
			"- **%s** `%s` at `%s`: %s\n",
			diagnostic.GetSeverity().String(),
			markdownText(diagnostic.GetRuleCode()),
			markdownText(diagnosticLocation(diagnostic.GetLocation())),
			markdownText(diagnostic.GetMessage()),
		)
		fmt.Fprintf(output, "  - Authority: `%s`\n", markdownText(diagnostic.GetAuthority().String()))
		fmt.Fprintf(output, "  - Reference: `%s`\n", markdownText(diagnostic.GetAuthorityReference()))
		fmt.Fprintf(output, "  - Remediation: %s\n", markdownText(diagnostic.GetRemediation()))
	}
	fmt.Fprintln(output, "")
}

func renderMethodTiming(output *strings.Builder, method *optionsv1alpha1.MethodOptionsSpec) {
	timing := method.GetTiming()
	if timing == nil {
		return
	}
	if responseTimeout := timing.GetResponseTimeout(); responseTimeout != nil {
		fmt.Fprintf(output, "    - Response timeout: `%s`\n", durationText(responseTimeout.GetSeconds(), responseTimeout.GetNanos()))
	}
	if retryDelay := timing.GetRetryDelay(); retryDelay != nil {
		fmt.Fprintf(output, "    - Retry delay: `%s`\n", durationText(retryDelay.GetSeconds(), retryDelay.GetNanos()))
	}
}

func renderPublication(output *strings.Builder, publication *optionsv1alpha1.PublicationPolicy, indent string) {
	fmt.Fprintf(output, "%s- Publication: `%s`\n", indent, publication.GetKind().String())
	if cycle := publication.GetCycle(); cycle != nil {
		fmt.Fprintf(output, "%s  - Cycle: `%s`\n", indent, durationText(cycle.GetSeconds(), cycle.GetNanos()))
	}
	if debounce := publication.GetDebounce(); debounce != nil {
		fmt.Fprintf(output, "%s  - Debounce: `%s`\n", indent, durationText(debounce.GetSeconds(), debounce.GetNanos()))
	}
	if retention := publication.GetRetention(); retention != nil {
		fmt.Fprintf(output, "%s  - Retention: `%s`\n", indent, durationText(retention.GetSeconds(), retention.GetNanos()))
	}
	if epsilon := publication.GetEpsilon(); epsilon != nil {
		fmt.Fprintf(output, "%s  - Epsilon: `%d/%d`\n", indent, epsilon.GetNumerator(), epsilon.GetDenominator())
	}
}

func durationText(seconds int64, nanos int32) string {
	return fmt.Sprintf("%ds %dns", seconds, nanos)
}

func durationRangeText(value *optionsv1alpha1.DurationRange) string {
	if value == nil {
		return "(unspecified)"
	}
	minimum := value.GetMinimum()
	maximum := value.GetMaximum()
	return fmt.Sprintf("%s..%s", durationText(minimum.GetSeconds(), minimum.GetNanos()), durationText(maximum.GetSeconds(), maximum.GetNanos()))
}

func memberDirectionText(direction manifestv1alpha1.MemberDirection) string {
	return strings.TrimPrefix(direction.String(), "MEMBER_DIRECTION_")
}

func effectiveTransportText(transport manifestv1alpha1.EffectiveTransportClassification) string {
	return strings.TrimPrefix(transport.String(), "EFFECTIVE_TRANSPORT_CLASSIFICATION_")
}

func tpUseResultText(result manifestv1alpha1.TpUseResult) string {
	return strings.TrimPrefix(result.String(), "TP_USE_RESULT_")
}

func providerSelectionModeText(mode optionsv1alpha1.ProviderSelectionMode) string {
	return strings.TrimPrefix(mode.String(), "PROVIDER_SELECTION_MODE_")
}

func deliveryModeText(mode optionsv1alpha1.EventGroupDeliveryMode) string {
	return strings.TrimPrefix(mode.String(), "EVENT_GROUP_DELIVERY_MODE_")
}

func consumerEndpointPortText(endpoint *optionsv1alpha1.ConsumerEndpoint) string {
	if endpoint == nil {
		return "(unspecified)"
	}
	switch port := endpoint.GetPort().(type) {
	case *optionsv1alpha1.ConsumerEndpoint_FixedPort:
		return fmt.Sprintf("%d", port.FixedPort)
	case *optionsv1alpha1.ConsumerEndpoint_DynamicPortRange:
		if port.DynamicPortRange == nil {
			return "(unspecified)"
		}
		return fmt.Sprintf("%d..%d", port.DynamicPortRange.GetMinimumPort(), port.DynamicPortRange.GetMaximumPort())
	default:
		return "(unspecified)"
	}
}

func effectiveEndpointText(endpoint *manifestv1alpha1.EffectiveEndpointManifest) string {
	if endpoint == nil {
		return ""
	}
	port := "(unspecified)"
	switch value := endpoint.GetPort().(type) {
	case *manifestv1alpha1.EffectiveEndpointManifest_FixedPort:
		port = fmt.Sprintf("%d", value.FixedPort)
	case *manifestv1alpha1.EffectiveEndpointManifest_DynamicPortRange:
		if value.DynamicPortRange != nil {
			port = fmt.Sprintf("%d..%d", value.DynamicPortRange.GetMinimumPort(), value.DynamicPortRange.GetMaximumPort())
		}
	}
	return fmt.Sprintf("%s %s %s port %s", endpoint.GetEndpointName(), endpoint.GetTransport().String(), endpoint.GetAddress(), port)
}

func multicastEndpointSuffix(endpoint *optionsv1alpha1.MulticastEndpoint) string {
	if endpoint == nil {
		return ""
	}
	return fmt.Sprintf("; multicast %s port %d", endpoint.GetAddress(), endpoint.GetPort())
}

func markdownStringList(values []string) string {
	formatted := make([]string, len(values))
	for index, value := range values {
		formatted[index] = "`" + markdownText(value) + "`"
	}
	return strings.Join(formatted, ", ")
}

func renderDocumentationURIs(output *strings.Builder, indent string, uris []string) {
	if len(uris) == 0 {
		return
	}
	fmt.Fprintf(output, "%s- Documentation URIs: %s\n", indent, markdownStringList(uris))
}

func optionalDescription(description string) string {
	if description == "" {
		return ""
	}
	return ": " + markdownText(description)
}

func semanticVersion(version *optionsv1alpha1.SemanticVersion) string {
	if version == nil {
		return "(unspecified)"
	}
	value := fmt.Sprintf("%d.%d.%d", version.GetMajor(), version.GetMinor(), version.GetPatch())
	if version.GetPrerelease() != "" {
		value += "-" + version.GetPrerelease()
	}
	if version.GetBuildMetadata() != "" {
		value += "+" + version.GetBuildMetadata()
	}
	return value
}

func encodedSizeText(size *manifestv1alpha1.EncodedSize) string {
	if size == nil {
		return "unknown"
	}
	if size.GetExactBytes() != 0 || (size.GetMinimumBytes() == 0 && size.GetMaximumBytes() == 0) {
		return fmt.Sprintf("`%d` bytes", size.GetExactBytes())
	}
	return fmt.Sprintf("`%d..%d` bytes", size.GetMinimumBytes(), size.GetMaximumBytes())
}

func diagnosticLocation(location *manifestv1alpha1.SourceLocation) string {
	if location == nil || location.GetImportPath() == "" {
		return "(unknown location)"
	}
	if location.GetLine() == 0 {
		return location.GetImportPath()
	}
	return fmt.Sprintf("%s:%d:%d", location.GetImportPath(), location.GetLine(), location.GetColumn())
}

func markdownText(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "`", "\\`")
	value = strings.ReplaceAll(value, "\r\n", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "\r", " ")
	return value
}

func sortedDiagnostics(diagnostics []*manifestv1alpha1.Diagnostic) []*manifestv1alpha1.Diagnostic {
	ordered := append([]*manifestv1alpha1.Diagnostic(nil), diagnostics...)
	sort.Slice(ordered, func(i, j int) bool {
		left, right := ordered[i], ordered[j]
		if left.GetRuleCode() != right.GetRuleCode() {
			return left.GetRuleCode() < right.GetRuleCode()
		}
		if left.GetSeverity() != right.GetSeverity() {
			return left.GetSeverity() < right.GetSeverity()
		}
		if diagnosticLocation(left.GetLocation()) != diagnosticLocation(right.GetLocation()) {
			return diagnosticLocation(left.GetLocation()) < diagnosticLocation(right.GetLocation())
		}
		return left.GetMessage() < right.GetMessage()
	})
	return ordered
}

func renderHTMLProvenance(output *strings.Builder, provenance *manifestv1alpha1.ManifestProvenance) {
	sectionStart(output, "Provenance", "provenance")
	if provenance == nil {
		muted(output, "No provenance was recorded.")
		sectionEnd(output)
		return
	}
	rows := [][2]string{
		{"Manifest format", fmt.Sprintf("%d", provenance.GetManifestFormatVersion())},
		{"Options API", provenance.GetOptionsApiVersion()},
		{"Input FDS SHA-256", provenance.GetInputFdsSha256()},
		{"Tool version", provenance.GetToolVersion()},
		{"Normative source commit", provenance.GetNormativeSourceCommit()},
	}
	if provenance.GetSourceRevision() != "" {
		rows = append(rows, [2]string{"Source revision", provenance.GetSourceRevision()})
	}
	propertyTable(output, rows)
	sectionEnd(output)
}

func renderHTMLProfile(output *strings.Builder, profile *manifestv1alpha1.ProfileManifest) {
	sectionStart(output, "Deployment profile", "profile")
	if profile == nil {
		muted(output, "No deployment profile was selected.")
		sectionEnd(output)
		return
	}
	propertyTable(output, [][2]string{
		{"Name", profile.GetProfileName()},
		{"Version", semanticVersion(profile.GetVersion())},
		{"UDP payload budget", fmt.Sprintf("%d bytes", profile.GetUdpApplicationPayloadBudgetBytes())},
	})
	renderHTMLDocumentationURIs(output, profile.GetDocumentationUris())
	if len(profile.GetProviderInstances()) > 0 {
		fmt.Fprint(output, "<h3>Provider identities</h3><div class=\"card-grid\">")
		for _, provider := range profile.GetProviderInstances() {
			instance := provider.GetInstance()
			fmt.Fprintf(output, "<article class=\"card\"><h4>%s</h4>", htmlText(instance.GetProviderName()))
			propertyTable(output, [][2]string{
				{"Service / instance", fmt.Sprintf("%d / %d", instance.GetServiceId(), instance.GetInstanceId())},
				{"Lineage", instance.GetServiceLineage().GetId()},
				{"Default endpoint", instance.GetDefaultEndpointName()},
				{"Maximum application payload", fmt.Sprintf("%d bytes", provider.GetMaximumApplicationPayloadBytes())},
				{"Maximum SOME/IP message", fmt.Sprintf("%d bytes", provider.GetMaximumSomeipMessageBytes())},
			})
			if len(instance.GetEndpoints()) > 0 {
				fmt.Fprint(output, "<h5>Endpoints</h5><table><thead><tr><th>Name</th><th>Transport</th><th>Address</th><th>Port</th></tr></thead><tbody>")
				for _, endpoint := range instance.GetEndpoints() {
					fmt.Fprintf(output, "<tr><td>%s</td><td>%s</td><td>%s</td><td>%d</td></tr>", htmlText(endpoint.GetName()), htmlText(endpoint.GetTransport().String()), htmlText(endpoint.GetAddress()), endpoint.GetPort())
				}
				output.WriteString("</tbody></table>")
			}
			if len(provider.GetServiceVersionBindings()) > 0 {
				fmt.Fprint(output, "<h5>Service-version bindings</h5><table><thead><tr><th>Service</th><th>Lineage</th><th>Major</th><th>Payload</th></tr></thead><tbody>")
				for _, binding := range provider.GetServiceVersionBindings() {
					fmt.Fprintf(output, "<tr><td>%s</td><td>%s</td><td>%d</td><td>%d bytes</td></tr>", htmlText(binding.GetBinding().GetService().GetFqn()), htmlText(binding.GetServiceLineage().GetId()), binding.GetBinding().GetInterfaceMajorVersion(), binding.GetMaximumApplicationPayloadBytes())
				}
				output.WriteString("</tbody></table>")
				for _, binding := range provider.GetServiceVersionBindings() {
					renderHTMLProviderStaticContract(output, binding)
				}
			}
			output.WriteString("</article>")
		}
		output.WriteString("</div>")
	} else {
		fmt.Fprint(output, "<h3>Provider identities</h3>")
		muted(output, "None.")
	}
	renderHTMLConsumerBindings(output, profile.GetConsumerBindings())
	sectionEnd(output)
}

func renderHTMLProviderStaticContract(output *strings.Builder, binding *manifestv1alpha1.ProviderServiceVersionBindingManifest) {
	if binding == nil {
		return
	}
	fmt.Fprintf(output, "<h5>Static SD/TP contract: %s major %d</h5>", htmlText(binding.GetBinding().GetService().GetFqn()), binding.GetBinding().GetInterfaceMajorVersion())
	offer := binding.GetServiceDiscoveryOffer()
	if offer == nil {
		muted(output, "No static Service Discovery offer.")
	} else {
		propertyTable(output, serviceDiscoveryTimingRows(offer.GetTiming(), strings.Join(offer.GetEndpointNames(), ", ")))
	}
	policies := binding.GetEventGroupDeliveryPolicies()
	if len(policies) == 0 && offer != nil {
		policies = offer.GetEventGroupDeliveryPolicies()
	}
	if len(policies) == 0 {
		muted(output, "No event-group delivery policies.")
	} else {
		fmt.Fprint(output, "<h6>Event-group delivery policies</h6><table><thead><tr><th>Event group</th><th>Mode</th><th>Multicast destination</th></tr></thead><tbody>")
		for _, policy := range policies {
			fmt.Fprintf(output, "<tr><td>%s</td><td>%s</td><td>%s</td></tr>", htmlText(policy.GetEventGroupName()), htmlText(deliveryModeText(policy.GetMode())), htmlText(multicastEndpointText(policy.GetMulticastEndpoint())))
		}
		output.WriteString("</tbody></table>")
	}
	if configurations := binding.GetMemberTpConfigurations(); len(configurations) == 0 {
		muted(output, "No provider TP declarations.")
	} else {
		fmt.Fprint(output, "<h6>Provider TP declarations</h6><table><thead><tr><th>Member ID</th><th>TP when needed</th></tr></thead><tbody>")
		for _, configuration := range configurations {
			fmt.Fprintf(output, "<tr><td>%d</td><td>%t</td></tr>", configuration.GetMemberId(), configuration.GetTpWhenNeeded())
		}
		output.WriteString("</tbody></table>")
	}
	renderHTMLTPAnalyses(output, binding.GetMemberTpAnalysis(), "Derived provider TP analysis")
}

func renderHTMLConsumerBindings(output *strings.Builder, consumers []*manifestv1alpha1.ConsumerServiceBindingManifest) {
	fmt.Fprint(output, "<h3>Consumer bindings</h3>")
	if len(consumers) == 0 {
		muted(output, "None.")
		return
	}
	fmt.Fprint(output, "<div class=\"card-grid\">")
	for _, consumer := range consumers {
		binding := consumer.GetBinding()
		fmt.Fprintf(output, "<article class=\"card\"><h4>%s</h4>", htmlText(binding.GetConsumerName()))
		propertyTable(output, [][2]string{
			{"Target service", binding.GetService().GetFqn()},
			{"Lineage", binding.GetServiceLineage().GetId()},
			{"Interface major", fmt.Sprintf("%d", binding.GetInterfaceMajorVersion())},
			{"Default local endpoint", binding.GetDefaultEndpointName()},
			{"Provider selector", providerSelectorText(consumer.GetProviderSelector())},
		})
		renderHTMLProviderCandidates(output, consumer.GetProviderSelector())
		if endpoints := consumer.GetLocalEndpoints(); len(endpoints) == 0 {
			muted(output, "No local endpoints.")
		} else {
			fmt.Fprint(output, "<h5>Local endpoints</h5><table><thead><tr><th>Name</th><th>Transport</th><th>Address</th><th>Port</th></tr></thead><tbody>")
			for _, endpoint := range endpoints {
				fmt.Fprintf(output, "<tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>", htmlText(endpoint.GetName()), htmlText(endpoint.GetTransport().String()), htmlText(endpoint.GetAddress()), htmlText(consumerEndpointPortText(endpoint)))
			}
			output.WriteString("</tbody></table>")
		}
		if overrides := binding.GetMemberEndpointOverrides(); len(overrides) > 0 {
			fmt.Fprint(output, "<h5>Member local-endpoint overrides</h5><table><thead><tr><th>Member ID</th><th>Endpoint</th></tr></thead><tbody>")
			for _, override := range overrides {
				fmt.Fprintf(output, "<tr><td>%d</td><td>%s</td></tr>", override.GetMemberId(), htmlText(override.GetEndpointName()))
			}
			output.WriteString("</tbody></table>")
		}
		propertyTable(output, serviceDiscoveryTimingRows(consumer.GetFindTiming(), ""))
		renderHTMLConsumerSubscriptions(output, consumer.GetSubscriptions())
		renderHTMLTPAnalyses(output, consumer.GetInheritedMemberTpAnalysis(), "Inherited provider TP analysis")
		output.WriteString("</article>")
	}
	output.WriteString("</div>")
}

func serviceDiscoveryTimingRows(timing *optionsv1alpha1.ServiceDiscoveryTiming, endpoints string) [][2]string {
	if timing == nil {
		return [][2]string{{"Service Discovery timing", "none"}}
	}
	rows := make([][2]string, 0, 7)
	if endpoints != "" {
		rows = append(rows, [2]string{"Advertised endpoints", endpoints})
	}
	rows = append(rows, [2]string{"Live TTL", fmt.Sprintf("%d seconds", timing.GetLiveTtlSeconds())})
	if timing.GetInitialDelay() == nil && timing.GetRepetitionsBaseDelay() == nil && timing.GetRepetitionsMax() == 0 && timing.GetRequestResponseDelay() == nil && timing.GetCyclicOfferDelay() == nil {
		return append(rows, [2]string{"Timing detail", "TTL-only"})
	}
	if initial := timing.GetInitialDelay(); initial != nil {
		rows = append(rows, [2]string{"Initial delay", durationRangeText(initial)})
	}
	if delay := timing.GetRepetitionsBaseDelay(); delay != nil {
		rows = append(rows, [2]string{"Repetitions", fmt.Sprintf("%d every %s", timing.GetRepetitionsMax(), durationText(delay.GetSeconds(), delay.GetNanos()))})
	}
	if delay := timing.GetRequestResponseDelay(); delay != nil {
		rows = append(rows, [2]string{"Request-response delay", durationRangeText(delay)})
	}
	if delay := timing.GetCyclicOfferDelay(); delay != nil {
		rows = append(rows, [2]string{"Cyclic delay", durationText(delay.GetSeconds(), delay.GetNanos())})
	}
	return rows
}

func renderHTMLProviderCandidates(output *strings.Builder, selector *manifestv1alpha1.ResolvedProviderSelectorManifest) {
	if selector == nil || len(selector.GetCandidates()) == 0 {
		muted(output, "No resolved provider candidates.")
		return
	}
	fmt.Fprint(output, "<h5>Resolved provider candidates</h5><table><thead><tr><th>Provider</th><th>Service / instance</th><th>Service</th><th>Major</th><th>Lineage</th></tr></thead><tbody>")
	for _, candidate := range selector.GetCandidates() {
		fmt.Fprintf(output, "<tr><td>%s</td><td>%d / %d</td><td>%s</td><td>%d</td><td>%s</td></tr>", htmlText(candidate.GetProviderName()), candidate.GetServiceId(), candidate.GetInstanceId(), htmlText(candidate.GetService().GetFqn()), candidate.GetInterfaceMajorVersion(), htmlText(candidate.GetServiceLineage().GetId()))
	}
	output.WriteString("</tbody></table>")
}

func renderHTMLConsumerSubscriptions(output *strings.Builder, subscriptions []*manifestv1alpha1.ConsumerSubscriptionManifest) {
	if len(subscriptions) == 0 {
		muted(output, "No subscriptions.")
		return
	}
	fmt.Fprint(output, "<h5>Subscriptions</h5><table><thead><tr><th>Event group</th><th>ID</th><th>TTL</th><th>Local endpoints</th><th>Initial data</th><th>Delivery policy</th></tr></thead><tbody>")
	for _, subscription := range subscriptions {
		authored := subscription.GetSubscription()
		delivery := "Provider-specific or unavailable"
		if policy := subscription.GetDeliveryPolicy(); policy != nil {
			delivery = deliveryModeText(policy.GetMode()) + multicastEndpointSuffix(policy.GetMulticastEndpoint())
		}
		fmt.Fprintf(output, "<tr><td>%s</td><td>%d</td><td>%d seconds</td><td>%s</td><td>%t</td><td>%s</td></tr>", htmlText(authored.GetEventGroupName()), subscription.GetEventGroupId(), authored.GetTtlSeconds(), htmlText(strings.Join(authored.GetLocalEndpointNames(), ", ")), subscription.GetInitialDataRequested(), htmlText(delivery))
	}
	output.WriteString("</tbody></table>")
}

func renderHTMLTPAnalyses(output *strings.Builder, analyses []*manifestv1alpha1.ProviderMemberTpAnalysisManifest, title string) {
	fmt.Fprintf(output, "<h6>%s</h6>", htmlText(title))
	if len(analyses) == 0 {
		muted(output, "None.")
		return
	}
	fmt.Fprint(output, "<table><thead><tr><th>Member ID</th><th>Direction</th><th>Payload bounds</th><th>Transport</th><th>Endpoint</th><th>TP when needed</th><th>Result</th><th>UDP budget</th><th>Runtime TP/session</th></tr></thead><tbody>")
	for _, analysis := range analyses {
		for _, direction := range analysis.GetDirections() {
			endpoint := effectiveEndpointText(direction.GetEndpoint())
			if endpoint == "" {
				endpoint = "No candidate-specific endpoint"
			}
			fmt.Fprintf(output, "<tr><td>%d</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td>%t</td><td>%s</td><td>%d bytes</td><td>%t</td></tr>", analysis.GetMemberId(), htmlText(memberDirectionText(direction.GetDirection())), htmlText(encodedSizeText(direction.GetEncodedSize())), htmlText(effectiveTransportText(direction.GetEffectiveTransport())), htmlText(endpoint), direction.GetTpWhenNeeded(), htmlText(tpUseResultText(direction.GetTpUseResult())), direction.GetUnsegmentedUdpPayloadBudgetBytes(), direction.GetRuntimeTpSessionHandlingRequired())
		}
	}
	output.WriteString("</tbody></table>")
}

func providerSelectorText(selector *manifestv1alpha1.ResolvedProviderSelectorManifest) string {
	if selector == nil || selector.GetSelector() == nil {
		return "None"
	}
	authored := selector.GetSelector()
	switch value := authored.GetSelector().(type) {
	case *optionsv1alpha1.ProviderSelector_NamedProviderName:
		return "Named provider: " + value.NamedProviderName
	case *optionsv1alpha1.ProviderSelector_AnyMatchingInstance:
		return fmt.Sprintf("Any matching instance: %t; mode: %s", value.AnyMatchingInstance, providerSelectionModeText(authored.GetSelectionMode()))
	default:
		return "Unspecified"
	}
}

func multicastEndpointText(endpoint *optionsv1alpha1.MulticastEndpoint) string {
	if endpoint == nil {
		return ""
	}
	return fmt.Sprintf("%s:%d", endpoint.GetAddress(), endpoint.GetPort())
}

func renderHTMLServices(output *strings.Builder, services []*manifestv1alpha1.ServiceManifest) {
	sectionStart(output, "Services", "services")
	if len(services) == 0 {
		muted(output, "None.")
		sectionEnd(output)
		return
	}
	for _, service := range services {
		fmt.Fprintf(output, "<article class=\"card entity\"><h3>%s</h3>", htmlText(service.GetService().GetFqn()))
		propertyTable(output, [][2]string{{"Lineage", service.GetServiceLineage().GetId()}, {"Interface version", fmt.Sprintf("%d.%d", service.GetInterfaceMajorVersion(), service.GetInterfaceMinorVersion())}})
		renderHTMLDocumentationURIs(output, service.GetDocumentationUris())
		if len(service.GetMembers()) > 0 {
			fmt.Fprint(output, "<h4>Members</h4><table><thead><tr><th>ID</th><th>Name</th><th>Kind</th><th>Contract</th><th>Directional payload bounds</th></tr></thead><tbody>")
			for _, member := range service.GetMembers() {
				contract := ""
				if method := member.GetMethodOptions(); method != nil {
					contract = method.GetKind().String() + "; " + method.GetResponsePolicy().String()
				}
				fmt.Fprintf(output, "<tr><td>%d</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>", member.GetMemberId(), htmlText(member.GetName()), htmlText(member.GetKind()), htmlText(contract), htmlText(directionalPayloadText(member.GetDirectionalPayloads())))
			}
			output.WriteString("</tbody></table>")
		}
		output.WriteString("</article>")
	}
	sectionEnd(output)
}

func directionalPayloadText(payloads []*manifestv1alpha1.MemberDirectionalPayloadManifest) string {
	values := make([]string, 0, len(payloads))
	for _, payload := range payloads {
		values = append(values, memberDirectionText(payload.GetDirection())+": "+encodedSizeText(payload.GetEncodedSize()))
	}
	return strings.Join(values, "; ")
}

func renderHTMLPayloads(output *strings.Builder, payloads []*manifestv1alpha1.PayloadTypeManifest) {
	sectionStart(output, "Payload types", "payloads")
	if len(payloads) == 0 {
		muted(output, "None.")
		sectionEnd(output)
		return
	}
	for _, payload := range payloads {
		fmt.Fprintf(output, "<article class=\"card entity\"><h3>%s</h3>", htmlText(payload.GetType().GetFqn()))
		layout := payload.GetEffectiveLayout()
		byteOrder, lengthWidth, extensible := "", "", ""
		if layout != nil {
			byteOrder, lengthWidth, extensible = layout.GetByteOrder().String(), layout.GetLengthFieldWidth().String(), fmt.Sprintf("%t", layout.GetExtensible())
		}
		propertyTable(output, [][2]string{{"Encoded size", encodedSizeText(payload.GetEncodedSize())}, {"Byte order", byteOrder}, {"Struct length", lengthWidth}, {"Extensible", extensible}})
		renderHTMLDocumentationURIs(output, payload.GetOptions().GetDocumentationUris())
		fmt.Fprint(output, "<h4>Layout hierarchy</h4><table><thead><tr><th>Kind</th><th>Name</th><th>Wire order / type</th><th>Encoded size</th><th>Alignment / padding</th></tr></thead><tbody>")
		for _, element := range payload.GetLayoutElements() {
			if member := element.GetMember(); member != nil {
				alignment := ""
				if value := member.GetAlignment(); value != nil {
					alignment = fmt.Sprintf("%d bytes; proven %t; offsets %d..%d", value.GetRequiredAlignmentBytes(), value.GetProven(), value.GetMinimumOffsetBytes(), value.GetMaximumOffsetBytes())
				}
				fmt.Fprintf(output, "<tr><td>Member</td><td>%s</td><td>%d / field %d</td><td>%s</td><td>%s</td></tr>", htmlText(member.GetName()), member.GetOptions().GetWireOrder(), member.GetFieldNumber(), encodedSizeText(member.GetEncodedSize()), htmlText(alignment))
			}
			if union := element.GetUnion(); union != nil {
				fmt.Fprintf(output, "<tr class=\"subheader\"><td>Union</td><td>value</td><td>effective slot %d bytes</td><td>%s</td><td>header %s / length %s</td></tr>", union.GetEffectiveValueSlotBytes(), encodedSizeText(union.GetEncodedSize()), htmlText(union.GetEffectiveLayout().GetHeaderOrder().String()), htmlText(union.GetEffectiveLayout().GetLengthFieldWidth().String()))
				for _, alternative := range union.GetAlternatives() {
					padding := fmt.Sprintf("declared %d", alternative.GetDeclaredPaddingBytes())
					if alternative.GetPadToEffectiveSlot() {
						padding += "; dynamic zero-fill"
					}
					if alternative.GetFixedDerivedZeroFillBytes() != 0 {
						padding += fmt.Sprintf("; derived zero-fill %d", alternative.GetFixedDerivedZeroFillBytes())
					}
					fmt.Fprintf(output, "<tr class=\"nested\"><td>Alternative</td><td>%s</td><td>type %d</td><td>%s</td><td>%s</td></tr>", htmlText(alternative.GetName()), alternative.GetTypeId(), encodedSizeText(alternative.GetEncodedSize()), htmlText(padding))
				}
			}
		}
		output.WriteString("</tbody></table></article>")
	}
	sectionEnd(output)
}

func renderHTMLEnums(output *strings.Builder, enums []*manifestv1alpha1.EnumManifest) {
	sectionStart(output, "Enum types", "enums")
	if len(enums) == 0 {
		muted(output, "None.")
		sectionEnd(output)
		return
	}
	for _, enum := range enums {
		fmt.Fprintf(output, "<article class=\"card entity\"><h3>%s</h3>", htmlText(enum.GetType().GetFqn()))
		propertyTable(output, [][2]string{{"Wire width", fmt.Sprintf("%d bits", enum.GetOptions().GetWireWidthBits())}, {"Closed", fmt.Sprintf("%t", enum.GetOptions().GetClosed())}})
		renderHTMLDocumentationURIs(output, enum.GetOptions().GetDocumentationUris())
		fmt.Fprint(output, "<table><thead><tr><th>Number</th><th>Name</th><th>Role</th></tr></thead><tbody>")
		for _, value := range enum.GetValues() {
			role := ""
			if value.GetOptions().GetNonSendablePlaceholder() {
				role = "Non-sendable placeholder"
			}
			fmt.Fprintf(output, "<tr><td>%d</td><td>%s</td><td>%s</td></tr>", value.GetNumber(), htmlText(value.GetName()), htmlText(role))
		}
		output.WriteString("</tbody></table></article>")
	}
	sectionEnd(output)
}

func renderHTMLUnknownOptions(output *strings.Builder, options []*manifestv1alpha1.UnknownOption) {
	sectionStart(output, "Preserved unknown options", "unknown-options")
	if len(options) == 0 {
		muted(output, "None.")
		sectionEnd(output)
		return
	}
	output.WriteString("<table><thead><tr><th>Location</th><th>Extendee</th><th>Field</th><th>Full name</th><th>Wire type</th></tr></thead><tbody>")
	for _, option := range options {
		fmt.Fprintf(output, "<tr><td>%s</td><td>%s</td><td>%d</td><td>%s</td><td>%d</td></tr>", htmlText(diagnosticLocation(option.GetLocation())), htmlText(option.GetExtendeeKind().String()), option.GetFieldNumber(), htmlText(option.GetFullName()), option.GetWireType())
	}
	output.WriteString("</tbody></table>")
	sectionEnd(output)
}

func renderHTMLDiagnostics(output *strings.Builder, diagnostics []*manifestv1alpha1.Diagnostic) {
	sectionStart(output, "Diagnostics", "diagnostics")
	ordered := sortedDiagnostics(diagnostics)
	if len(ordered) == 0 {
		muted(output, "No findings.")
		sectionEnd(output)
		return
	}
	output.WriteString("<div class=\"findings\"><table><thead><tr><th>Severity</th><th>Rule</th><th>Location</th><th>Finding</th><th>Remediation</th></tr></thead><tbody>")
	for _, diagnostic := range ordered {
		severity := diagnostic.GetSeverity().String()
		fmt.Fprintf(output, "<tr class=\"severity-%s\"><td><span class=\"badge\">%s</span></td><td><code>%s</code></td><td><code>%s</code></td><td><strong>%s</strong><br><span class=\"muted\">%s: %s</span></td><td>%s</td></tr>", severityClass(diagnostic.GetSeverity()), htmlText(severityLabel(severity)), htmlText(diagnostic.GetRuleCode()), htmlText(diagnosticLocation(diagnostic.GetLocation())), htmlText(diagnostic.GetMessage()), htmlText(diagnostic.GetAuthority().String()), htmlText(diagnostic.GetAuthorityReference()), htmlText(diagnostic.GetRemediation()))
	}
	output.WriteString("</tbody></table></div>")
	sectionEnd(output)
}

func sectionStart(output *strings.Builder, title, id string) {
	fmt.Fprintf(output, "<section id=\"%s\"><div class=\"section-heading\"><span class=\"section-index\">%s</span><h2>%s</h2></div>", htmlText(id), htmlText(fmt.Sprintf("%02d", sectionNumber(id))), htmlText(title))
}
func sectionEnd(output *strings.Builder) { output.WriteString("</section>") }
func muted(output *strings.Builder, value string) {
	fmt.Fprintf(output, "<p class=\"muted\">%s</p>", htmlText(value))
}
func propertyTable(output *strings.Builder, rows [][2]string) {
	output.WriteString("<table class=\"properties\"><tbody>")
	for _, row := range rows {
		fmt.Fprintf(output, "<tr><th>%s</th><td>%s</td></tr>", htmlText(row[0]), htmlText(row[1]))
	}
	output.WriteString("</tbody></table>")
}
func renderHTMLDocumentationURIs(output *strings.Builder, uris []string) {
	if len(uris) == 0 {
		return
	}
	output.WriteString("<p class=\"muted\"><strong>Documentation:</strong> ")
	for index, uri := range uris {
		if index > 0 {
			output.WriteString(", ")
		}
		fmt.Fprintf(output, "<code>%s</code>", htmlText(uri))
	}
	output.WriteString("</p>")
}
func htmlText(value string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\"", "&#34;", "'", "&#39;").Replace(value)
}
func severityClass(severity manifestv1alpha1.DiagnosticSeverity) string {
	switch severity {
	case manifestv1alpha1.DiagnosticSeverity_DIAGNOSTIC_SEVERITY_ERROR:
		return "error"
	case manifestv1alpha1.DiagnosticSeverity_DIAGNOSTIC_SEVERITY_WARNING:
		return "warning"
	default:
		return "info"
	}
}
func severityLabel(value string) string {
	value = strings.TrimPrefix(value, "DIAGNOSTIC_SEVERITY_")
	return strings.Title(strings.ToLower(value))
}
func diagnosticCounts(diagnostics []*manifestv1alpha1.Diagnostic) (errors, warnings, infos int) {
	for _, diagnostic := range diagnostics {
		switch diagnostic.GetSeverity() {
		case manifestv1alpha1.DiagnosticSeverity_DIAGNOSTIC_SEVERITY_ERROR:
			errors++
		case manifestv1alpha1.DiagnosticSeverity_DIAGNOSTIC_SEVERITY_WARNING:
			warnings++
		default:
			infos++
		}
	}
	return
}
func sectionNumber(id string) int {
	switch id {
	case "provenance":
		return 1
	case "profile":
		return 2
	case "services":
		return 3
	case "payloads":
		return 4
	case "enums":
		return 5
	case "unknown-options":
		return 6
	case "diagnostics":
		return 7
	default:
		return 0
	}
}

const htmlReportCSS = `
:root { color-scheme: light dark; font-family: Inter, ui-sans-serif, system-ui, sans-serif; }
body { margin: 0; background: #f3f6fa; color: #172033; }
main { max-width: 1180px; margin: 0 auto; padding: 32px 24px 64px; }
.hero { padding: 28px 30px; border-radius: 18px; color: #fff; background: linear-gradient(135deg, #172554, #2563eb); box-shadow: 0 12px 30px #1e3a8a33; }
.hero h1 { margin: 4px 0 8px; font-size: clamp(1.5rem, 3vw, 2.3rem); overflow-wrap: anywhere; }
.eyebrow, .muted { color: #64748b; } .hero .eyebrow, .hero .muted { color: #dbeafe; }
.eyebrow { margin: 0; font-size: .75rem; font-weight: 800; letter-spacing: .12em; text-transform: uppercase; }
.summary-grid { display: grid; grid-template-columns: repeat(4, 1fr); gap: 14px; margin: 20px 0 34px; }
.summary-card { padding: 18px; border: 1px solid #dbe2ea; border-left: 5px solid #94a3b8; border-radius: 12px; background: #fff; box-shadow: 0 3px 12px #0f172a0b; }
.summary-card strong { display: block; font-size: 1.8rem; } .summary-card span { color: #64748b; font-size: .85rem; }
.summary-card.error, .severity-error { border-left-color: #dc2626; } .summary-card.warning, .severity-warning { border-left-color: #d97706; } .summary-card.info { border-left-color: #2563eb; }
section { margin-top: 34px; } .section-heading { display: flex; align-items: center; gap: 12px; margin-bottom: 14px; } h2 { margin: 0; font-size: 1.35rem; } h3 { overflow-wrap: anywhere; } h4, h5 { margin-bottom: 10px; }
.section-index { display: grid; place-items: center; width: 30px; height: 30px; border-radius: 9px; color: #1d4ed8; background: #dbeafe; font-size: .78rem; font-weight: 800; }
.card-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(310px, 1fr)); gap: 16px; } .card { margin: 14px 0; padding: 18px; border: 1px solid #dbe2ea; border-radius: 12px; background: #fff; box-shadow: 0 3px 12px #0f172a0b; } .entity { padding: 22px; }
table { width: 100%; border-collapse: collapse; margin: 10px 0 18px; background: #fff; border: 1px solid #dbe2ea; border-radius: 10px; overflow: hidden; } th, td { padding: 10px 12px; border-bottom: 1px solid #e5eaf0; text-align: left; vertical-align: top; overflow-wrap: anywhere; } th { color: #475569; background: #f8fafc; font-size: .78rem; text-transform: uppercase; letter-spacing: .04em; } tr:last-child td { border-bottom: 0; } .properties { margin: 0 0 16px; } .properties th { width: 28%; text-transform: none; letter-spacing: 0; }
.nested td:first-child { padding-left: 32px; color: #64748b; } .subheader td { background: #eff6ff; font-weight: 700; }
.findings table { font-size: .92rem; } .findings tr.severity-error { background: #fff1f2; } .findings tr.severity-warning { background: #fffbeb; } .findings tr.severity-info { background: #eff6ff; } .badge { display: inline-block; padding: 4px 8px; border-radius: 999px; color: #fff; background: #2563eb; font-size: .72rem; font-weight: 800; text-transform: uppercase; } .severity-error .badge { background: #dc2626; } .severity-warning .badge { background: #d97706; }
code { font: .86em ui-monospace, SFMono-Regular, Menlo, monospace; color: #334155; } a { color: #1d4ed8; }
@media (max-width: 720px) { main { padding: 16px 12px 40px; } .summary-grid { grid-template-columns: repeat(2, 1fr); } table { display: block; overflow-x: auto; white-space: normal; } }
@media (prefers-color-scheme: dark) { body { background: #0b1220; color: #e2e8f0; } .card, table, .summary-card { background: #111827; border-color: #334155; } th { background: #1e293b; color: #cbd5e1; } td { border-color: #334155; } .muted, .summary-card span { color: #94a3b8; } .findings tr.severity-error { background: #3f1d25; } .findings tr.severity-warning { background: #3b2b12; } .findings tr.severity-info { background: #172554; } code { color: #cbd5e1; } }
`
