# Phase 2.1 schema fixtures

These fixtures are descriptor-driven inputs for the FDS backend and canonical
renderers. They establish the source shape and expected normalized contract;
the schemas themselves do not perform semantic validation.

| Fixture | Expected normalization result | Catalog rule / manifest evidence |
| --- | --- | --- |
| `UnequalFixedZeroWidthUnion` | Warning, not failure. The effective value slot is two bytes; the one-byte alternative carries one fixed derived zero-fill byte. | `SIP-UNION-002`; ordered `UnionManifest.alternatives`, `effective_value_slot_bytes`, and `fixed_derived_zero_fill_bytes`. |
| `BoundedDynamicZeroWidthUnion` | Warning, not failure. The dynamic alternative uses a data-dependent fill-to-slot rule rather than a fictitious fixed padding count. | `SIP-UNION-002`; `pad_to_effective_slot`. |
| `LengthPrefixedUnionPadding` | Accept. The first alternative has three author-declared padding bytes, included in the length-prefixed value extent. | `SIP-UNION-001`; `declared_padding_bytes`. |
| `AlignmentWarningPayload` | Warning, not failure. The second element is not provably four-byte aligned from the full SOME/IP message start. | `SIP-LAYOUT-003`; `AlignmentAnalysis`. |
| `phase-2-1-valid` | Accept. One provider identity binds two different major versions from the same immutable lineage while sharing the Service/Instance IDs. | `SIP-PROFILE-001`; ordered provider service-version bindings. |
| `phase-2-1-invalid` | Errors for duplicate lineage/major binding and binding a foreign lineage to the identity. | `SIP-PROFILE-001`; the invalid identity must not yield a final manifest. |
| `PhaseTwoOneReservedIdentifierService` | Error for the reserved method/event ID zero. | `SIP-ID-001`. |
| `sd-tp-valid` | Accept. Two equivalent UDP providers support an exact and an `ALL_MATCHES` wildcard consumer, static offer/find timing, unicast and multicast delivery, fixed and dynamic consumer ports, and subscriptions. | `SIP-SD-001` through `SIP-SD-004`; normalized candidates, endpoints, timing, delivery policies, and subscription intent. |
| `StaticSDTPService` | Accept. It derives `NEVER_NEEDED`, `TP_NOT_CONFIGURED`, `MAY_BE_NEEDED`, and `REQUIRED` for request, response, exception, and notification payload directions. | `SIP-TP-001` through `SIP-TP-004`; directional payload bounds and provider/inherited TP analysis. |

## Canonical JSON contract

The valid fixture's protobuf JSON must be deterministic with no
descriptor-derived absolute paths or timestamps. At a minimum, it preserves
these semantic orderings:

1. provider identities by Service ID then Instance ID;
2. service-version bindings by lineage, major version, then service FQN;
3. payload layout elements by wire order;
4. union alternatives by type ID; and
5. SD offers, delivery policies, TP configurations, consumer bindings,
   endpoints, subscriptions, candidates, and directed analyses by their
   canonical semantic ordering; and
6. diagnostics and unknown options by the ordering defined in `plan.md`.

The golden artifacts show effective big-endian union headers, resolved layout
facts, zero-fill choice, service lineage, provider bindings, static SD facts,
and directed TP analysis. They do not invent runtime decoder values, runtime
SD selection, or TP session behavior.