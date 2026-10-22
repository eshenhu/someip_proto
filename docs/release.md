# Release procedure

The tag-triggered [`Release`](../.github/workflows/release.yml) workflow is the
only supported publication path. It reruns the full Linux/macOS release gates,
packages Linux and macOS amd64/arm64 CLI binaries, generates release FDS and
report artifacts, emits checksums and an SPDX SBOM, publishes the protobuf
module to BSR, attests the assets, and creates the GitHub Release.

Before triggering a tag, a repository administrator must complete the remote
controls in [`repository-settings.md`](repository-settings.md), create the
protected `release` environment, and provide a `BUF_TOKEN` secret scoped to
that environment with publish access to `buf.build/eshenhu/someip_proto`.

To stage a release locally without publishing it:

```sh
RELEASE_VERSION=v0.1.0 make release-package
```

Set `SOURCE_REVISION` to an explicit logical revision when staging a release
outside GitHub Actions. The staged `dist/` directory contains the four archives,
the SmartWorkshopLighting core and static SD/TP FDS/manifest/report artifact
suites, and `SHA256SUMS`. The release workflow refreshes `SHA256SUMS` after
creating the SPDX SBOM, so it covers every uploaded asset.