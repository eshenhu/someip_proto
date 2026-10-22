# someip_proto v0.1 proposal

## Purpose

`someip_proto` is an independent public project that models static SOME/IP
interfaces, payload layouts, and deployment metadata using annotated proto3
source. It must not copy, import, or depend on DDAD, SystemProductsProtocol,
SPP, internal Bazel tooling, proprietary fixtures, or their source materials.

The stable protobuf namespace is `io.github.someip_proto`. Initial public
packages are experimental `v1alpha1`; stable `v1` packages require an explicit
migration and compatibility commitment.

## Artifact authority

1. Annotated proto3 files are the sole source authority.
2. A complete, source-aware binary `FileDescriptorSet` is the canonical
   frontend-to-backend handoff.
3. A typed `.manifest.binpb` is the normalized backend and future-generator
   contract.
4. Canonical protobuf JSON, Markdown, and self-contained static HTML are
   deterministic renderings of that manifest.
5. Compatibility re-normalizes baseline and current FDS artifacts using the
   same tool rules.

## v0.1 includes

- Static SOME/IP RPC service, method, event, field, event-group,
   provider-instance, endpoint, timing, and deployment metadata.
- Static Service Discovery provider offers, consumer selectors/endpoints,
   find and offer timing, subscriptions, and explicit unicast or multicast
   event-group delivery policy.
- Provider-authored per-member `tp_when_needed` capability and derived
   direction-specific UDP/TCP TP-use analysis.
- Full static payload-layout metadata with deterministic validation and size
  analysis.
- FDS parsing, option resolution, semantic validation, profile manifests,
  reports, and comparison.

## v0.1 excludes

- Byte-compatible encoders/decoders, PDU/header schemas, network transport,
   gRPC stubs, SD/TP PDU encoding or decoding, SD state machines/timers,
   TP segmentation/reassembly/session handling/traffic shaping, E2E, security/
   key exchange, and runtime API/code generation.
- Direct FIDL, Franca, or CDEPL parsing; client-authored TP controls, adaptive
   or client multicast, generic validity fields, and sentinel-value conventions.
- DDAD/SPP integration, internal re-encoding pipelines, internal Bazel labels
  and toolchains, and any DDAD-derived public fixture.

## Normative authority and policy

The normative baseline is Open SOME/IP 2025-12, declared source version
`25-12`, at commit `94e4ea932e50273897a014bc57f7ae89f1816b84`; see
[`references.md`](references.md). Each validation rule is classified as either
`NORMATIVE`, with a pinned source section identifier, or `PROJECT_POLICY`, with
an ADR rationale. Diagnostics and reports expose that classification.

## Planned public interfaces

The command-line interface will provide `validate`, `manifest`, `report`, and
`compare`. Its stable integration surfaces will be the CLI plus FDS and
manifest artifacts/schemas. The Go library remains experimental.

The project will use Bzlmod, Buf, native protobuf rules, rules_go, and a
project-owned hermetic descriptor-build rule. It will support Linux and macOS
and publish checksummed release artifacts.

## Governance

Contributions require DCO sign-off and one maintainer review. Releases use
protected GitHub Actions workflows and immutable pre-1 SemVer tags, beginning
with the intended `v0.1.0`. See [`repository-settings.md`](repository-settings.md)
for the required public-repository controls.
