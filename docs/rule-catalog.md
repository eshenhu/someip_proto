# Rule catalog

This catalog records the first public static contract derived from the pinned
Open SOME/IP source. It contains only short disposition summaries; it neither
copies normative wording nor treats untracked source trees as authority.

## Review basis and interpretation

- **Source:** Open SOME/IP `25-12` at
  `94e4ea932e50273897a014bc57f7ae89f1816b84`; see
  [`references.md`](references.md).
- **Review unit:** every `feat_req` identifier in the four tracked source files,
  including information entries, figures, and identifier tables needed to
  interpret a requirement.
- **Precedence:** a precise identifier row below overrides the complete-source
  range row for its file. A complete-source range row is an explicit record for
  every identifier selected by that range, not an uncataloged remainder.
- **Authority:** only `NORMATIVE` and `PROJECT_POLICY` are diagnostic authority
  values. `OUT_OF_SCOPE_*` is a catalog disposition, never an authority value.
- **Implementation:** stable rule codes are implemented in the FDS backend and
  remain part of the public static contract.

## Dispositions

| Disposition | Meaning |
| --- | --- |
| `STATIC_VALIDATOR` | The backend reports the stated stable diagnostic when the resolved FDS violates the rule. |
| `STATIC_MANIFEST` | The backend records the resolved fact in the normalized manifest. |
| `OUT_OF_SCOPE_RUNTIME` | Requires a codec, transport, request lifecycle, or runtime receiver behavior. |
| `OUT_OF_SCOPE_SD` | Requires SOME/IP Service Discovery messages, subscriptions, or SD state. |
| `OUT_OF_SCOPE_TP` | Requires SOME/IP-TP behavior. |
| `OUT_OF_SCOPE_E2E` | Requires E2E-protection behavior. |
| `PROJECT_POLICY` | Deliberate v0.1 restriction with an ADR rationale. |

## Complete-source range records

These records account for every reviewed identifier before precise static rows
are applied. They intentionally include headings, information entries, figures,
and tables because those supply the context for the precise rows.

| Source selector | Default disposition | Authority | Evidence / owner |
| --- | --- | --- | --- |
| `src/someip-rpc.rst#feat_req_someip_*` | `OUT_OF_SCOPE_RUNTIME` | `NORMATIVE`, source identifier | The tool leaves header construction, codec execution, request/session lifecycle, UDP/TCP bindings, magic cookies, and error processing out of scope. |
| `src/someip-ids.rst#feat_req_someipids_*` | `STATIC_MANIFEST` | `NORMATIVE`, source identifier | The tool preserves the interpreted reserved-ID facts used by ID diagnostics. |
| `src/someip-compat.rst#feat_req_someipcompat_*` | `OUT_OF_SCOPE_RUNTIME` | `NORMATIVE`, source identifier | Runtime decoder tolerance and client/server behavior are not claimed by v0.1. |
| `src/someip-sd.rst#feat_req_someipsd_*` | `OUT_OF_SCOPE_RUNTIME` | `NORMATIVE`, source identifier | The static validator records topology, timing, subscriptions, and delivery policy; it emits no SD PDU, timer, state machine, or runtime discovery behavior. |
| `src/someip-tp.rst#feat_req_someiptp_*` | `OUT_OF_SCOPE_RUNTIME` | `NORMATIVE`, source identifier | The static validator derives provider-owned TP capability facts but emits no TP header, segment, session, reassembly, or traffic-shaping behavior. |

## Static validator and manifest records

| Sources | v0.1 disposition | Authority reference | Rule / severity | Manifest evidence |
| --- | --- | --- | --- | --- |
| `src/someip-rpc.rst#feat_req_someip_538`, `#feat_req_someip_539`, `#feat_req_someip_541`, `#feat_req_someip_542`, `#feat_req_someip_543`, `#feat_req_someip_544`, `#feat_req_someip_545`, `#feat_req_someip_546`, `#feat_req_someip_547`, `#feat_req_someip_625`; `src/someip-ids.rst#feat_req_someipids_505`, `#feat_req_someipids_529`, `#feat_req_someipids_555`, `#feat_req_someipids_636` | `STATIC_VALIDATOR` | `NORMATIVE` | `SIP-ID-001` / error | Provider IDs, member IDs, and event-group IDs with source locations. |
| `src/someip-rpc.rst#feat_req_someip_626` | `STATIC_VALIDATOR` | `NORMATIVE` | `SIP-ID-002` / warning | Recommended method/event allocation classification. |
| `src/someip-rpc.rst#feat_req_someip_42`, `#feat_req_someip_675`, `#feat_req_someip_224` | `STATIC_VALIDATOR` | `NORMATIVE` | `SIP-LAYOUT-001` / error | Declared and effective payload byte order; union header byte order is always big endian. |
| `src/someip-rpc.rst#feat_req_someip_169`, `#feat_req_someip_230`, `#feat_req_someip_574`, `#feat_req_someip_575`, `#feat_req_someip_600`, `#feat_req_someip_602` | `STATIC_VALIDATOR` | `NORMATIVE` | `SIP-LAYOUT-002` / error | Ordered layout elements, explicit padding, length framing, extensibility, and highest wire order. |
| `src/someip-rpc.rst#feat_req_someip_711`, `#feat_req_someip_577` | `STATIC_VALIDATOR` | `NORMATIVE` | `SIP-LAYOUT-003` / warning | Alignment analysis measured from the complete SOME/IP message start. |
| `src/someip-rpc.rst#feat_req_someip_172`, `#feat_req_someip_817`, `#feat_req_someip_623` | `STATIC_VALIDATOR` | `NORMATIVE` | `SIP-TYPE-001` / error | Scalar representation and conservative encoded bounds. |
| `src/someip-rpc.rst#feat_req_someip_233`, `#feat_req_someip_234`, `#feat_req_someip_235`, `#feat_req_someip_237`, `#feat_req_someip_238`, `#feat_req_someip_239`, `#feat_req_someip_562`, `#feat_req_someip_581`, `#feat_req_someip_582`, `#feat_req_someip_639`, `#feat_req_someip_640`, `#feat_req_someip_641`, `#feat_req_someip_642`, `#feat_req_someip_662`, `#feat_req_someip_666`, `#feat_req_someip_687`, `#feat_req_someip_800` | `STATIC_VALIDATOR` | `NORMATIVE` | `SIP-STRING-001` / error | Declared and effective prefix width plus BOM, terminator, and fixed-fill consequences. |
| `src/someip-rpc.rst#feat_req_someip_241`, `#feat_req_someip_243`, `#feat_req_someip_252`, `#feat_req_someip_254`, `#feat_req_someip_257`, `#feat_req_someip_259`, `#feat_req_someip_260`, `#feat_req_someip_621` | `STATIC_VALIDATOR` | `NORMATIVE` | `SIP-ARRAY-001` / error | Array prefix width, fixed/max item bounds, and encoded-size bounds. |
| `src/someip-rpc.rst#feat_req_someip_651`, `#feat_req_someip_799`, `#feat_req_someip_689`, `#feat_req_someip_690`, `#feat_req_someip_691` | `STATIC_VALIDATOR` | `NORMATIVE` | `SIP-ENUM-001` / error | Enum width/closure and bitfield width/named-bit metadata. |
| `src/someip-rpc.rst#feat_req_someip_263`, `#feat_req_someip_264`, `#feat_req_someip_272`, `#feat_req_someip_273`, `#feat_req_someip_274`, `#feat_req_someip_563`, `#feat_req_someip_564`, `#feat_req_someip_566`, `#feat_req_someip_571`, `#feat_req_someip_572`, `#feat_req_someip_573`, `#feat_req_someip_583` | `STATIC_VALIDATOR` | `NORMATIVE` | `SIP-UNION-001` / error; `SIP-UNION-002` / warning | Ordered union manifest, fixed big-endian headers, alternatives, declared padding, effective slot, and derived zero-fill rule. |
| `src/someip-rpc.rst#feat_req_someip_141`, `#feat_req_someip_345`, `#feat_req_someip_348`, `#feat_req_someip_367`, `#feat_req_someip_106`, `#feat_req_someip_107`, `#feat_req_someip_371`, `#feat_req_someip_597`, `#feat_req_someip_654`, `#feat_req_someip_655` | `STATIC_VALIDATOR` | `NORMATIVE` | `SIP-MEMBER-001` / error | Method kind, return-code metadata, exception payload, and response-policy contract. |
| `src/someip-rpc.rst#feat_req_someip_59`, `#feat_req_someip_631`, `#feat_req_someip_632`, `#feat_req_someip_633`, `#feat_req_someip_634`, `#feat_req_someip_635` | `STATIC_VALIDATOR` | `NORMATIVE` | `SIP-FIELD-001` / error | Resolved field accessor and notifier member contracts plus event-group memberships. |
| `src/someip-rpc.rst#feat_req_someip_32`, `#feat_req_someip_35`, `#feat_req_someip_36`, `#feat_req_someip_318`, `#feat_req_someip_636`, `#feat_req_someip_967`; `src/someip-compat.rst#feat_req_someipcompat_714`, `#feat_req_someipcompat_799`, `#feat_req_someipcompat_800`, `#feat_req_someipcompat_803` | `STATIC_MANIFEST` and `STATIC_VALIDATOR` | `NORMATIVE` | `SIP-PROFILE-001` / error; `SIP-COMPAT-001` / error | Provider identity, service lineage, ordered major-version bindings, endpoint references, payload budget, and interface-major relationship. |
| Static forward-evolution analysis | `STATIC_VALIDATOR` and `STATIC_MANIFEST` | `PROJECT_POLICY`, `docs/adr/0004-static-authoring-policy.md` | `SIP-COMPAT-002` / error | Extensible-struct boundaries, length-delimited elements, and append-only evolution markers; runtime decoder obligations remain excluded. |
| `src/someip-rpc.rst#feat_req_someip_168`, `#feat_req_someip_601`; `src/someip-compat.rst#feat_req_someipcompat_1198`, `#feat_req_someipcompat_1199`, `#feat_req_someipcompat_1200` | `OUT_OF_SCOPE_RUNTIME` | `NORMATIVE` | none | The catalog records decoder skipping, longer-element acceptance, and missing-value behavior as exclusions without inventing a default-value convention. |
| `src/someip-sd.rst#feat_req_someipsd_780` | `STATIC_VALIDATOR` and `STATIC_MANIFEST` | `NORMATIVE` | `SIP-SD-001` / error | Provider offers, consumer identities, endpoint topology, selectors, and deterministic provider candidates. |
| `src/someip-sd.rst#feat_req_someipsd_63` | `STATIC_VALIDATOR` and `STATIC_MANIFEST` | `NORMATIVE` | `SIP-SD-002` / error | Valid uint24 TTL values plus nonnegative integral-millisecond offer/find timing facts. |
| `src/someip-sd.rst#feat_req_someipsd_322` | `STATIC_VALIDATOR` and `STATIC_MANIFEST` | `NORMATIVE` | `SIP-SD-003` / error | Event-group subscriptions, local endpoint references, TTL, and initial-data intent. |
| `src/someip-sd.rst#feat_req_someipsd_793` | `STATIC_VALIDATOR` and `STATIC_MANIFEST` | `NORMATIVE` | `SIP-SD-004` / error | Explicit unicast or multicast server delivery policy per offered event group. |
| `src/someip-tp.rst#feat_req_someiptp_788` | `STATIC_VALIDATOR` and `STATIC_MANIFEST` | `NORMATIVE` | `SIP-TP-001` / error | Provider member TP declaration integrity and canonical provider-owned capability facts. |
| `src/someip-tp.rst#feat_req_someiptp_760` | `STATIC_VALIDATOR` and `STATIC_MANIFEST` | `NORMATIVE` | `SIP-TP-002` / error | TP-disabled UDP payload-budget consistency and derived directional TP use. |

## Project-policy records

| Topic | Disposition | Authority reference | Rule / severity | Manifest evidence |
| --- | --- | --- | --- | --- |
| Nonempty annotated services and event groups | `PROJECT_POLICY` | `PROJECT_POLICY`, `docs/adr/0004-static-authoring-policy.md` | `SIP-POLICY-001` / error | Service members and event-group memberships. |
| Unary-RPC-only authoring and explicit member metadata | `PROJECT_POLICY` | `PROJECT_POLICY`, `docs/adr/0004-static-authoring-policy.md` | `SIP-POLICY-002` / error | Normalized dispatchable members. |
| Restricted serializable proto closure, explicit wire order/bounds, and nested multidimensional wrappers | `PROJECT_POLICY` | `PROJECT_POLICY`, `docs/adr/0004-static-authoring-policy.md` | `SIP-POLICY-003` / error | Reachable payload layouts and encoded bounds. |
| Static provider lineage and multi-major representation without runtime/SD demultiplexing | `PROJECT_POLICY` | `PROJECT_POLICY`, `docs/adr/0004-static-authoring-policy.md` | `SIP-POLICY-004` / error | Provider service-version bindings. |
| Static forward-evolution analysis without a runtime decoder claim | `PROJECT_POLICY` | `PROJECT_POLICY`, `docs/adr/0004-static-authoring-policy.md` | `SIP-COMPAT-002` / error | Extensible-struct boundaries, length-delimited elements, and append-only evolution markers. |
| Deprecated preliminary schema fields are not canonical Phase 2.1 semantic input or output | `PROJECT_POLICY` | `PROJECT_POLICY`, `docs/adr/0001-artifact-authority.md` | `SIP-ARTIFACT-002` / error | Canonical provider bindings and `layout_elements`; legacy fields remain only for Buf source compatibility. |
| FDS-only backend boundary, provenance, unknown-option preservation, and deterministic renderings | `PROJECT_POLICY` | `PROJECT_POLICY`, `docs/adr/0001-artifact-authority.md` | `SIP-ARTIFACT-001` / error | Provenance, source locations, unknown options, diagnostics, and stable ordering. |
| Complete provider TP declaration coverage for every UDP-capable dispatchable member | `PROJECT_POLICY` | `PROJECT_POLICY`, `docs/adr/0005-first-public-static-sd-tp-contract.md` | `SIP-TP-003` / error | One provider-authored `tp_when_needed` declaration and directed TP analysis per member. |
| Uniform TP contract for wildcard provider selector candidates | `PROJECT_POLICY` | `PROJECT_POLICY`, `docs/adr/0005-first-public-static-sd-tp-contract.md` | `SIP-TP-004` / error | One inherited member TP contract after candidate-equivalence validation. |

## Implementation contract

The implementation enforces every `STATIC_VALIDATOR` row using the cataloged
rule code, severity, authority, source location, and remediation. It
materializes every `STATIC_MANIFEST` row without requiring consumers to reapply
inheritance, prefix defaults, union recovery, layout evolution, static SD, or
provider TP rules. Core and static SD/TP fixture outcomes are recorded in
[`phase-2-1-fixtures.md`](phase-2-1-fixtures.md).