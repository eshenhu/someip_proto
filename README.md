# someip_proto

`someip_proto` is a public, MIT-licensed project for authoring static SOME/IP
interface and deployment metadata as annotated proto3 source.

The project will use a complete, source-aware protobuf `FileDescriptorSet` as
the strict frontend-to-backend boundary and a versioned normalized manifest as
the future generator contract. 

## Status

The repository has a checked-in `v0.1.0` release baseline. The initial public
APIs remain experimental packages under `io.github.someip_proto.*.v1alpha1`; no
stable `v1` compatibility commitment exists yet.

The intended public repository is
[`eshenhu/someip_proto`](https://github.com/eshenhu/someip_proto), and the
planned Go module path is `github.com/eshenhu/someip_proto`.

## Scope

The v0.1 proposal covers static SOME/IP RPC metadata, payload-layout metadata,
provider and consumer Service Discovery topology, provider-authored SOME/IP-TP
capability analysis, FileDescriptorSet validation, normalized manifests,
deterministic reports, and semantic compatibility comparison. It intentionally
excludes runtime codecs, network transport, SD/TP PDUs and state machines,
segmentation/reassembly, E2E, generated client bindings, and proprietary or
internal integrations.

See [`docs/proposal.md`](docs/proposal.md) for the normative project proposal,
[`docs/references.md`](docs/references.md) for external references, and
[`docs/adr/`](docs/adr/) for architectural decisions.

## Development

Phase 1 uses Bazel modules, Buf v1.73.0, and Go modules. After installing Bazel,
Go, and Buf, use `make check` to run the non-mutating format, lint, compatibility,
Bazel, Go, descriptor-smoke, and dependency-lock checks. Use `make buf-format` to
apply protobuf formatting locally.

See [`GUIDE.md`](GUIDE.md) for the normal schema-validation workflow, FDS
production, manifest/report generation, and compatibility comparison commands.
The non-normative [`SmartWorkshopLighting`](docs/smart-workshop-lighting.md)
examples provide provider-only core and complete static SD/TP profiles with
checked-in artifact baselines. [`docs/release.md`](docs/release.md) describes
the protected tag release procedure.

## Contributing and security

Contributions require DCO sign-off and maintainer review; see
[`CONTRIBUTING.md`](CONTRIBUTING.md). Please report security vulnerabilities
privately through GitHub Security Advisories as described in
[`SECURITY.md`](SECURITY.md).

## License

This project is licensed under the [MIT License](LICENSE).
