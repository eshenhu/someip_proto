package semantic

import manifestv1alpha1 "github.com/eshenhu/someip_proto/gen/go/io/github/someip_proto/manifest/v1alpha1"

type rule struct {
	code        string
	authority   manifestv1alpha1.RuleAuthority
	reference   string
	remediation string
}

var (
	ruleID = rule{
		code:        "SIP-ID-001",
		authority:   manifestv1alpha1.RuleAuthority_RULE_AUTHORITY_NORMATIVE,
		reference:   "src/someip-ids.rst#feat_req_someipids_505",
		remediation: "Use a non-reserved unsigned 16-bit identifier that is unique in its required scope.",
	}
	ruleIDRecommendation = rule{
		code:        "SIP-ID-002",
		authority:   manifestv1alpha1.RuleAuthority_RULE_AUTHORITY_NORMATIVE,
		reference:   "src/someip-rpc.rst#feat_req_someip_626",
		remediation: "Use the recommended method and event/notification identifier ranges, or document the deliberate allocation.",
	}
	ruleLayout = rule{
		code:        "SIP-LAYOUT-001",
		authority:   manifestv1alpha1.RuleAuthority_RULE_AUTHORITY_NORMATIVE,
		reference:   "src/someip-rpc.rst#feat_req_someip_42",
		remediation: "Declare a valid effective byte order and keep union headers in network byte order.",
	}
	ruleLayoutFraming = rule{
		code:        "SIP-LAYOUT-002",
		authority:   manifestv1alpha1.RuleAuthority_RULE_AUTHORITY_NORMATIVE,
		reference:   "src/someip-rpc.rst#feat_req_someip_169",
		remediation: "Declare contiguous wire order, complete framing, and explicit valid padding metadata.",
	}
	ruleAlignment = rule{
		code:        "SIP-LAYOUT-003",
		authority:   manifestv1alpha1.RuleAuthority_RULE_AUTHORITY_NORMATIVE,
		reference:   "src/someip-rpc.rst#feat_req_someip_577",
		remediation: "Add explicit padding or revise the layout so the declared alignment is provable from the SOME/IP message start.",
	}
	ruleType = rule{
		code:        "SIP-TYPE-001",
		authority:   manifestv1alpha1.RuleAuthority_RULE_AUTHORITY_NORMATIVE,
		reference:   "src/someip-rpc.rst#feat_req_someip_172",
		remediation: "Use a supported protobuf source type with its required SOME/IP representation and finite bound.",
	}
	ruleString = rule{
		code:        "SIP-STRING-001",
		authority:   manifestv1alpha1.RuleAuthority_RULE_AUTHORITY_NORMATIVE,
		reference:   "src/someip-rpc.rst#feat_req_someip_233",
		remediation: "Declare a valid string encoding, framing form, and finite encoded bound including required framing bytes.",
	}
	ruleArray = rule{
		code:        "SIP-ARRAY-001",
		authority:   manifestv1alpha1.RuleAuthority_RULE_AUTHORITY_NORMATIVE,
		reference:   "src/someip-rpc.rst#feat_req_someip_241",
		remediation: "Declare valid fixed or dynamic array framing and all required finite bounds.",
	}
	ruleEnum = rule{
		code:        "SIP-ENUM-001",
		authority:   manifestv1alpha1.RuleAuthority_RULE_AUTHORITY_NORMATIVE,
		reference:   "src/someip-rpc.rst#feat_req_someip_651",
		remediation: "Use a supported enum or bitfield width and valid numeric/named-bit metadata.",
	}
	ruleUnion = rule{
		code:        "SIP-UNION-001",
		authority:   manifestv1alpha1.RuleAuthority_RULE_AUTHORITY_NORMATIVE,
		reference:   "src/someip-rpc.rst#feat_req_someip_263",
		remediation: "Declare complete union headers, contiguous type IDs, finite alternative bounds, and valid padding.",
	}
	ruleUnionZeroWidth = rule{
		code:        "SIP-UNION-002",
		authority:   manifestv1alpha1.RuleAuthority_RULE_AUTHORITY_NORMATIVE,
		reference:   "src/someip-rpc.rst#feat_req_someip_583",
		remediation: "The normalizer will use the largest value slot and zero-fill shorter alternatives; consider a length field when this is undesirable.",
	}
	ruleMember = rule{
		code:        "SIP-MEMBER-001",
		authority:   manifestv1alpha1.RuleAuthority_RULE_AUTHORITY_NORMATIVE,
		reference:   "src/someip-rpc.rst#feat_req_someip_141",
		remediation: "Declare method kind, allowed return-code metadata, and a consistent response or exception policy.",
	}
	ruleField = rule{
		code:        "SIP-FIELD-001",
		authority:   manifestv1alpha1.RuleAuthority_RULE_AUTHORITY_NORMATIVE,
		reference:   "src/someip-rpc.rst#feat_req_someip_59",
		remediation: "Use same-service accessor RPCs with the required Empty/value-type signatures and valid notifier metadata.",
	}
	ruleProfile = rule{
		code:        "SIP-PROFILE-001",
		authority:   manifestv1alpha1.RuleAuthority_RULE_AUTHORITY_NORMATIVE,
		reference:   "src/someip-rpc.rst#feat_req_someip_32",
		remediation: "Declare a closed profile with valid provider identity, endpoints, lineage, bindings, and payload budget.",
	}
	ruleCompatibility = rule{
		code:        "SIP-COMPAT-001",
		authority:   manifestv1alpha1.RuleAuthority_RULE_AUTHORITY_NORMATIVE,
		reference:   "src/someip-compat.rst#feat_req_someipcompat_714",
		remediation: "Use the required interface and profile semantic-version transition for the resolved contract change.",
	}
	ruleStaticEvolution = rule{
		code:        "SIP-COMPAT-002",
		authority:   manifestv1alpha1.RuleAuthority_RULE_AUTHORITY_PROJECT_POLICY,
		reference:   "docs/adr/0004-static-authoring-policy.md",
		remediation: "Keep fixed layouts immutable; append only higher wire-order elements to extensible length-delimited structs.",
	}
	rulePolicyService = rule{
		code:        "SIP-POLICY-001",
		authority:   manifestv1alpha1.RuleAuthority_RULE_AUTHORITY_PROJECT_POLICY,
		reference:   "docs/adr/0004-static-authoring-policy.md",
		remediation: "Give each annotated service and declared event group at least one member.",
	}
	rulePolicyRPC = rule{
		code:        "SIP-POLICY-002",
		authority:   manifestv1alpha1.RuleAuthority_RULE_AUTHORITY_PROJECT_POLICY,
		reference:   "docs/adr/0004-static-authoring-policy.md",
		remediation: "Use unary RPCs and explicit SOME/IP method metadata for every RPC in an annotated service.",
	}
	rulePolicyPayload = rule{
		code:        "SIP-POLICY-003",
		authority:   manifestv1alpha1.RuleAuthority_RULE_AUTHORITY_PROJECT_POLICY,
		reference:   "docs/adr/0004-static-authoring-policy.md",
		remediation: "Use only the supported acyclic proto3 payload closure with explicit layout order and finite bounds.",
	}
	rulePolicyLineage = rule{
		code:        "SIP-POLICY-004",
		authority:   manifestv1alpha1.RuleAuthority_RULE_AUTHORITY_PROJECT_POLICY,
		reference:   "docs/adr/0004-static-authoring-policy.md",
		remediation: "Bind only service majors from the provider identity's declared immutable lineage.",
	}
	ruleArtifact = rule{
		code:        "SIP-ARTIFACT-001",
		authority:   manifestv1alpha1.RuleAuthority_RULE_AUTHORITY_PROJECT_POLICY,
		reference:   "docs/adr/0001-artifact-authority.md",
		remediation: "Provide a complete source-aware FDS and retain unknown runtime options in the normalized artifact.",
	}
	ruleArtifactCanonical = rule{
		code:        "SIP-ARTIFACT-002",
		authority:   manifestv1alpha1.RuleAuthority_RULE_AUTHORITY_PROJECT_POLICY,
		reference:   "docs/adr/0001-artifact-authority.md",
		remediation: "Use canonical provider service-version bindings and layout_elements instead of deprecated preliminary fields.",
	}
	ruleSDTopology = rule{
		code:        "SIP-SD-001",
		authority:   manifestv1alpha1.RuleAuthority_RULE_AUTHORITY_NORMATIVE,
		reference:   "src/someip-sd.rst#feat_req_someipsd_780",
		remediation: "Declare resolvable provider and consumer endpoints with a compatible service lineage, major version, selector, and address family.",
	}
	ruleSDTiming = rule{
		code:        "SIP-SD-002",
		authority:   manifestv1alpha1.RuleAuthority_RULE_AUTHORITY_NORMATIVE,
		reference:   "src/someip-sd.rst#feat_req_someipsd_63",
		remediation: "Use a nonzero uint24 TTL and complete, nonnegative integral-millisecond timing configuration where timing is declared.",
	}
	ruleSDSubscription = rule{
		code:        "SIP-SD-003",
		authority:   manifestv1alpha1.RuleAuthority_RULE_AUTHORITY_NORMATIVE,
		reference:   "src/someip-sd.rst#feat_req_someipsd_322",
		remediation: "Subscribe only to declared event groups with a valid TTL and the required client endpoints; request initial data only for a group containing a field notifier.",
	}
	ruleSDDelivery = rule{
		code:        "SIP-SD-004",
		authority:   manifestv1alpha1.RuleAuthority_RULE_AUTHORITY_NORMATIVE,
		reference:   "src/someip-sd.rst#feat_req_someipsd_793",
		remediation: "Declare exactly one explicit UNICAST or MULTICAST delivery policy for every offered event group and use a valid UDP multicast endpoint only for MULTICAST.",
	}
	ruleTPIntegrity = rule{
		code:        "SIP-TP-001",
		authority:   manifestv1alpha1.RuleAuthority_RULE_AUTHORITY_NORMATIVE,
		reference:   "src/someip-tp.rst#feat_req_someiptp_788",
		remediation: "Declare TP capability only once for each UDP-capable dispatchable member and do not apply TP declarations to TCP-only members.",
	}
	ruleTPBudget = rule{
		code:        "SIP-TP-002",
		authority:   manifestv1alpha1.RuleAuthority_RULE_AUTHORITY_NORMATIVE,
		reference:   "src/someip-tp.rst#feat_req_someiptp_760",
		remediation: "Enable tp_when_needed for an oversized UDP flow or reduce its maximum application payload to the unsegmented UDP budget.",
	}
	ruleTPCompleteness = rule{
		code:        "SIP-TP-003",
		authority:   manifestv1alpha1.RuleAuthority_RULE_AUTHORITY_PROJECT_POLICY,
		reference:   "docs/adr/0005-first-public-static-sd-tp-contract.md",
		remediation: "Declare exactly one provider-owned tp_when_needed value for every UDP-capable dispatchable member.",
	}
	ruleTPWildcard = rule{
		code:        "SIP-TP-004",
		authority:   manifestv1alpha1.RuleAuthority_RULE_AUTHORITY_PROJECT_POLICY,
		reference:   "docs/adr/0005-first-public-static-sd-tp-contract.md",
		remediation: "Use wildcard candidates with identical member TP flags and effective UDP/TCP classifications, or select one named provider.",
	}
)

var catalogedRules = []rule{
	ruleID,
	ruleIDRecommendation,
	ruleLayout,
	ruleLayoutFraming,
	ruleAlignment,
	ruleType,
	ruleString,
	ruleArray,
	ruleEnum,
	ruleUnion,
	ruleUnionZeroWidth,
	ruleMember,
	ruleField,
	ruleProfile,
	ruleCompatibility,
	ruleStaticEvolution,
	rulePolicyService,
	rulePolicyRPC,
	rulePolicyPayload,
	rulePolicyLineage,
	ruleArtifact,
	ruleArtifactCanonical,
	ruleSDTopology,
	ruleSDTiming,
	ruleSDSubscription,
	ruleSDDelivery,
	ruleTPIntegrity,
	ruleTPBudget,
	ruleTPCompleteness,
	ruleTPWildcard,
}
