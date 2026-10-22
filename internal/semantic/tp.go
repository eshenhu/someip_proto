package semantic

import (
	"fmt"
	"sort"

	manifestv1alpha1 "github.com/eshenhu/someip_proto/gen/go/io/github/someip_proto/manifest/v1alpha1"
	optionsv1alpha1 "github.com/eshenhu/someip_proto/gen/go/io/github/someip_proto/options/v1alpha1"
)

// normalizeProviderTP derives the provider-authored TP contract for every
// resolved service-version binding. It describes capability only; it neither
// configures nor implements TP transmission, segmentation, or reassembly.
func (n *normalizer) normalizeProviderTP(udpBudget uint64) {
	for _, binding := range n.providerBindings {
		n.normalizeProviderBindingTP(binding, udpBudget)
	}
}

func (n *normalizer) normalizeProviderBindingTP(binding *providerBindingInfo, udpBudget uint64) {
	if binding == nil || binding.binding == nil || binding.bindingManifest == nil || binding.service == nil {
		return
	}

	binding.bindingManifest.MemberTpConfigurations = canonicalMemberTPConfigurations(binding.binding.GetMemberTpConfigurations())
	configurations := make(map[uint32]*optionsv1alpha1.MemberTpConfiguration, len(binding.binding.GetMemberTpConfigurations()))
	members := make(map[uint32]*manifestv1alpha1.ServiceMemberManifest, len(binding.service.manifest.GetMembers()))
	for _, member := range binding.service.manifest.GetMembers() {
		members[member.GetMemberId()] = member
	}
	for _, configuration := range binding.binding.GetMemberTpConfigurations() {
		memberID := configuration.GetMemberId()
		if _, found := members[memberID]; !found {
			n.diagnostics.error(ruleTPIntegrity, binding.location, fmt.Sprintf("TP configuration references unknown member_id %d", memberID))
			continue
		}
		if _, found := configurations[memberID]; found {
			n.diagnostics.error(ruleTPIntegrity, binding.location, fmt.Sprintf("TP configuration for member_id %d is duplicated", memberID))
			continue
		}
		configurations[memberID] = configuration
	}

	analyses := make([]*manifestv1alpha1.ProviderMemberTpAnalysisManifest, 0, len(binding.service.manifest.GetMembers()))
	for _, member := range binding.service.manifest.GetMembers() {
		memberID := member.GetMemberId()
		endpoint := providerEndpointForMember(binding, memberID)
		configuration := configurations[memberID]
		usesUDP := endpoint != nil && endpoint.GetTransport() == optionsv1alpha1.TransportProtocol_TRANSPORT_PROTOCOL_UDP
		usesTCP := endpoint != nil && endpoint.GetTransport() == optionsv1alpha1.TransportProtocol_TRANSPORT_PROTOCOL_TCP

		if usesUDP && configuration == nil {
			n.diagnostics.error(ruleTPCompleteness, binding.location, fmt.Sprintf("UDP-capable member_id %d must declare exactly one TP configuration", memberID))
		}
		if usesTCP && configuration != nil {
			n.diagnostics.error(ruleTPIntegrity, binding.location, fmt.Sprintf("TP configuration for TCP-only member_id %d is not permitted", memberID))
		}

		analysis := &manifestv1alpha1.ProviderMemberTpAnalysisManifest{
			MemberId:     memberID,
			TpWhenNeeded: configuration.GetTpWhenNeeded(),
			Directions:   make([]*manifestv1alpha1.MemberDirectionTpAnalysisManifest, 0, len(binding.service.memberPayloadBounds[memberID])),
		}
		for _, payload := range binding.service.memberPayloadBounds[memberID] {
			direction := providerTPDirectionAnalysis(payload, endpoint, configuration, udpBudget)
			analysis.Directions = append(analysis.Directions, direction)
			if usesUDP && (configuration == nil || !configuration.GetTpWhenNeeded()) && payload.size.maximum > udpBudget {
				n.diagnostics.error(ruleTPBudget, binding.location, fmt.Sprintf("UDP member_id %d %s payload maximum %d exceeds unsegmented UDP budget %d while TP is not configured", memberID, payload.direction.String(), payload.size.maximum, udpBudget))
			}
		}
		analyses = append(analyses, analysis)
	}
	sort.Slice(analyses, func(i, j int) bool {
		return analyses[i].GetMemberId() < analyses[j].GetMemberId()
	})
	binding.bindingManifest.MemberTpAnalysis = analyses
}

func providerEndpointForMember(binding *providerBindingInfo, memberID uint32) *optionsv1alpha1.Endpoint {
	if binding == nil || binding.provider == nil {
		return nil
	}
	endpoint := binding.endpoints[binding.provider.GetDefaultEndpointName()]
	for _, override := range binding.binding.GetMemberEndpointOverrides() {
		if override.GetMemberId() != memberID {
			continue
		}
		if resolved := binding.endpoints[override.GetEndpointName()]; resolved != nil {
			endpoint = resolved
		}
		break
	}
	return endpoint
}

func providerTPDirectionAnalysis(
	payload memberDirectionalBound,
	endpoint *optionsv1alpha1.Endpoint,
	configuration *optionsv1alpha1.MemberTpConfiguration,
	udpBudget uint64,
) *manifestv1alpha1.MemberDirectionTpAnalysisManifest {
	analysis := &manifestv1alpha1.MemberDirectionTpAnalysisManifest{
		Direction:    payload.direction,
		EncodedSize:  payload.size.manifest(),
		TpWhenNeeded: configuration.GetTpWhenNeeded(),
	}
	if endpoint == nil {
		analysis.EffectiveTransport = manifestv1alpha1.EffectiveTransportClassification_EFFECTIVE_TRANSPORT_CLASSIFICATION_NOT_APPLICABLE
		analysis.TpUseResult = manifestv1alpha1.TpUseResult_TP_USE_RESULT_NOT_APPLICABLE
		return analysis
	}
	analysis.Endpoint = providerEndpointManifest(endpoint)
	switch endpoint.GetTransport() {
	case optionsv1alpha1.TransportProtocol_TRANSPORT_PROTOCOL_UDP:
		analysis.EffectiveTransport = manifestv1alpha1.EffectiveTransportClassification_EFFECTIVE_TRANSPORT_CLASSIFICATION_UDP
		analysis.UnsegmentedUdpPayloadBudgetBytes = udpBudget
		if configuration == nil || !configuration.GetTpWhenNeeded() {
			analysis.TpUseResult = manifestv1alpha1.TpUseResult_TP_USE_RESULT_TP_NOT_CONFIGURED
			return analysis
		}
		switch {
		case payload.size.maximum <= udpBudget:
			analysis.TpUseResult = manifestv1alpha1.TpUseResult_TP_USE_RESULT_NEVER_NEEDED
		case payload.size.minimum > udpBudget:
			analysis.TpUseResult = manifestv1alpha1.TpUseResult_TP_USE_RESULT_REQUIRED
			analysis.RuntimeTpSessionHandlingRequired = true
		default:
			analysis.TpUseResult = manifestv1alpha1.TpUseResult_TP_USE_RESULT_MAY_BE_NEEDED
			analysis.RuntimeTpSessionHandlingRequired = true
		}
	case optionsv1alpha1.TransportProtocol_TRANSPORT_PROTOCOL_TCP:
		analysis.EffectiveTransport = manifestv1alpha1.EffectiveTransportClassification_EFFECTIVE_TRANSPORT_CLASSIFICATION_TCP
		analysis.TpUseResult = manifestv1alpha1.TpUseResult_TP_USE_RESULT_NOT_APPLICABLE
	default:
		analysis.EffectiveTransport = manifestv1alpha1.EffectiveTransportClassification_EFFECTIVE_TRANSPORT_CLASSIFICATION_NOT_APPLICABLE
		analysis.TpUseResult = manifestv1alpha1.TpUseResult_TP_USE_RESULT_NOT_APPLICABLE
	}
	return analysis
}

func providerEndpointManifest(endpoint *optionsv1alpha1.Endpoint) *manifestv1alpha1.EffectiveEndpointManifest {
	if endpoint == nil {
		return nil
	}
	return &manifestv1alpha1.EffectiveEndpointManifest{
		EndpointName: endpoint.GetName(),
		Address:      endpoint.GetAddress(),
		Transport:    endpoint.GetTransport(),
		Port: &manifestv1alpha1.EffectiveEndpointManifest_FixedPort{
			FixedPort: endpoint.GetPort(),
		},
	}
}
