# User guide

This guide covers the normal authoring and validation workflow for
`someip_proto`. The project describes **static** SOME/IP contracts in annotated
proto3 files, including static Service Discovery topology and provider-authored
TP capability facts. It does not generate runtime codecs, transports, SD/TP
PDUs or state machines, or TP segmentation/reassembly behavior.

## Prerequisites

Install the versions pinned by the repository:

- Go `1.27.1`
- Bazel (Bazelisk is recommended)
- Buf `1.73.0`

Run the commands below from the repository root. The public protobuf module is
rooted at `proto/`.

## Quick validation workflow

Use this sequence after editing a `.proto` file:

1. Format the schema:

   ```sh
   make buf-format
   ```

2. Check protobuf style and schema rules:

   ```sh
   make buf-lint
   ```

3. Regenerate and verify Go protobuf bindings when a public schema changed:

   ```sh
   make go-generate
   make go-generate-check
   ```

4. Build a complete, source-aware `FileDescriptorSet` (FDS):

   ```sh
   bazel build //proto:phase_2_1_valid_descriptor
   ```

5. Run static SOME/IP validation against the FDS:

   ```sh
   descriptor_path="$(bazel cquery --output=files //proto:phase_2_1_valid_descriptor | tail -n 1)"
   go run ./cmd/someip-proto validate \
     --fds "$descriptor_path" \
     --profile phase-2-1-valid
   ```

The Phase 2.1 example deliberately emits alignment and zero-width-union
**warnings**. Warnings allow a successful exit; semantic errors return exit
code `1`.

The public non-normative example uses a separate focused descriptor target:

```sh
bazel build //proto:smart_workshop_lighting_descriptor
```

The complete static SD/TP example uses:

```sh
bazel build //proto:smart_workshop_lighting_sd_tp_descriptor
```

Its checked-in v0.1.0 core and SD/TP FDS, manifest, and report goldens are
verified together with:

```sh
make phase4-artifact-check
```

For a full repository check, use:

```sh
BASE_REF=HEAD make check
```

In CI or when comparing to an existing branch, set `BASE_REF` to the intended
Git baseline, such as `origin/main`. The breaking-schema check fails closed if
the baseline is unavailable. `ALLOW_MISSING_BUF_BASELINE=1` is only for an
explicit first-baseline bootstrap.

## What gets validated

Two complementary layers validate an annotated proto schema:

- **Buf** (`make buf-lint`, `make buf-breaking`) checks protobuf style and
  public-schema compatibility.
- **`someip-proto validate`** loads the complete FDS and checks resolved static
  SOME/IP semantics: identifiers, service/member contracts, profiles,
  provider/consumer endpoints, SD offers/find timing/subscriptions/delivery,
  provider TP declarations and directional TP-use analysis, payload layout,
  arrays, strings, enums, unions, encoded bounds, alignment, and project-policy
  restrictions.

The semantic tool accepts only a complete binary FDS with its import closure
and `SourceCodeInfo`. It does not read the original `.proto` files while
validating. Do not use a hand-trimmed descriptor set.

## Producing an FDS for your schemas

Add or update a `buf_descriptor` target in `proto/BUILD.bazel` (or another
package) whose `srcs` include the module configuration and every local proto in
the descriptor closure:

```starlark
buf_descriptor(
    name = "my_descriptor",
    module = "proto",
    srcs = [
        "buf.lock",
        "buf.yaml",
        "io/github/someip_proto/options/v1alpha1/someip_options.proto",
        "path/to/my_contract.proto",
    ],
)
```

Include every imported non-builtin local proto as well. Then build it and pass
the produced `.fds.binpb` path to the CLI:

```sh
bazel build //proto:my_descriptor
descriptor_path="$(bazel cquery --output=files //proto:my_descriptor | tail -n 1)"
go run ./cmd/someip-proto validate --fds "$descriptor_path"
```

Omit `--profile` to validate every deployment profile declared in the FDS. Use
`--profile NAME` to validate one selected profile.

## CLI commands

The development invocation is:

```sh
go run ./cmd/someip-proto <command> [flags]
```

Build a reusable binary with `bazel build //cmd/someip-proto`; use the produced
binary with the same arguments.

### Validate

```sh
go run ./cmd/someip-proto validate \
  --fds contract.fds.binpb \
  --profile my-profile \
  --format text
```

`validate` supports `text` (default) and `json` diagnostics. It writes to
standard output by default; use `--output path` to write a file.

### Emit a normalized manifest

A manifest is available only when semantic validation has no errors:

```sh
go run ./cmd/someip-proto manifest \
  --fds contract.fds.binpb \
  --profile my-profile \
  --format json \
  --output contract.manifest.json
```

Supported formats are `binpb` (default) and `json`.

### Emit a report

```sh
go run ./cmd/someip-proto report \
  --fds contract.fds.binpb \
  --profile my-profile \
  --format html \
  --output contract.report.html
```

Supported formats are `markdown` (default), `html`, and `json`. Output-file
writes are atomic.

### Compare two contract revisions

Compare two source-aware FDS artifacts under the same selected profile:

```sh
go run ./cmd/someip-proto compare \
  --baseline baseline.fds.binpb \
  --current current.fds.binpb \
  --profile my-profile \
  --format markdown \
  --output compatibility-report.md
```

Supported comparison formats are `text`, `json`, `markdown` (default), and
`html`. Compatibility errors return exit code `1`.

## Input, output, and safety flags

All commands that accept `--fds` also accept `--input` and `--descriptor-set`
as aliases. Pass `--fds -` to read the binary FDS from standard input.

The tool applies conservative FDS limits. Override them only for a known,
trusted large descriptor set:

- `--max-input-bytes`
- `--max-files`
- `--max-descriptors`
- `--max-descriptor-depth`
- `--max-source-locations`
- `--max-source-path-elements`

Add `--tool-version` and optional `--source-revision` when recording artifact
provenance. Provenance values must be logical identifiers, not absolute paths or
URIs.

Exit codes are:

| Code | Meaning |
| --- | --- |
| `0` | Success (warnings may have been reported). |
| `1` | Semantic or compatibility failure. |
| `2` | Invalid command usage or malformed FDS input. |
| `3` | Internal tool failure. |

Use `go run ./cmd/someip-proto help` for the command summary and
`go run ./cmd/someip-proto <command> --help` for flags.

## Additional developer checks

| Command | Purpose |
| --- | --- |
| `make bazel-test` | Run the complete Bazel test suite. |
| `make fds-smoke` | Verify the descriptor-producing Bazel rule. |
| `make go-test go-vet` | Run Go tests and static analysis. |
| `make go-fuzz-smoke` | Run seeded loader and semantic fuzz smoke tests. |
| `make lock-check` | Verify Bazel/Buf locks and provenance-ledger digest. |

## More detail

- [`docs/rule-catalog.md`](docs/rule-catalog.md) maps every implemented rule to
  its authority and stable diagnostic code.
- [`docs/phase-2-1-fixtures.md`](docs/phase-2-1-fixtures.md) explains the
  included core and static SD/TP fixture scenarios.
- [`docs/proposal.md`](docs/proposal.md) defines the project scope and explicit
  runtime exclusions.
- [`CONTRIBUTING.md`](CONTRIBUTING.md) covers contribution and DCO requirements.
