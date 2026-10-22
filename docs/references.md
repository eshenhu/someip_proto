# External references

This document records the external normative basis of `someip_proto` without
copying its text, figures, or tables.

## Open SOME/IP 2025-12

| Field | Value |
| --- | --- |
| Title | Open SOME/IP Specification, release 2025-12 |
| Declared source version | `25-12` |
| Immutable source | <https://github.com/some-ip-com/open-someip-spec/tree/94e4ea932e50273897a014bc57f7ae89f1816b84> |
| Pinned commit | `94e4ea932e50273897a014bc57f7ae89f1816b84` |
| Release confirmation | <https://github.com/some-ip-com/open-someip-spec/commit/94e4ea932e50273897a014bc57f7ae89f1816b84> |
| Publication index | <https://some-ip.com/standards.shtml> |
| Retrieved | 2026-09-22 |
| License/source context | <https://github.com/some-ip-com/open-someip-spec> |

Review the tracked files only at the pinned commit. Phase 2.1 reviewed these
immutable sources and their identifiers:

- [`src/someip-rpc.rst`](https://github.com/some-ip-com/open-someip-spec/blob/94e4ea932e50273897a014bc57f7ae89f1816b84/src/someip-rpc.rst)
- [`src/someip-ids.rst`](https://github.com/some-ip-com/open-someip-spec/blob/94e4ea932e50273897a014bc57f7ae89f1816b84/src/someip-ids.rst)
- [`src/someip-compat.rst`](https://github.com/some-ip-com/open-someip-spec/blob/94e4ea932e50273897a014bc57f7ae89f1816b84/src/someip-compat.rst)
- [`src/someip-sd.rst`](https://github.com/some-ip-com/open-someip-spec/blob/94e4ea932e50273897a014bc57f7ae89f1816b84/src/someip-sd.rst)
- [`src/someip-tp.rst`](https://github.com/some-ip-com/open-someip-spec/blob/94e4ea932e50273897a014bc57f7ae89f1816b84/src/someip-tp.rst)

The project records the resulting disposition of every reviewed identifier in
[`rule-catalog.md`](rule-catalog.md). The tool release provenance carries this
same commit, rather than inferring it from a working tree.

For the first public contract, statically authorable SD topology, timing,
subscription, delivery-policy, and provider TP-capability facts are validated
or materialized. SD/TP PDU formats and runtime behavior remain explicit
exclusions.

## Citation rules

Every validator rule classified as `NORMATIVE` must cite the relevant pinned
source section by path and requirement or section identifier, for example:

- `src/someip-rpc.rst`, requirement `feat_req_someip_*`
- `src/someip-ids.rst`, requirement `feat_req_someipids_*`
- `src/someip-compat.rst`, requirement `feat_req_someipcompat_*`
- `src/someip-sd.rst`, requirement `feat_req_someipsd_*`
- `src/someip-tp.rst`, requirement `feat_req_someiptp_*`

A rule classified as `PROJECT_POLICY` must instead cite its rationale and the
applicable project ADR. Reports and diagnostics must expose this authority
classification. No normative text, figures, or tables are to be copied into
this repository.

## Non-normative context

The project uses protobuf only as an authoring and metadata format. The Open
SOME/IP specification remains the external normative authority for applicable
SOME/IP protocol requirements; project policy defines the deliberate v0.1
scope and tooling boundaries.
