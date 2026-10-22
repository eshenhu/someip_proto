# Required GitHub repository settings

Phase 0 records these remote-side settings because repository administration was
deliberately deferred during local bootstrap. Apply them before making the
repository public or accepting contributions.

## Identity and default branch

- Create or use the public repository
  [`eshenhu/someip_proto`](https://github.com/eshenhu/someip_proto).
- Add `https://github.com/eshenhu/someip_proto.git` as the `origin` remote.
- Push the initial documentation-only commit.
- Set `main` as the GitHub default branch after it exists. The local repository
  currently has no configured remote and remains on its bootstrap branch until
  this remote-side step is completed.

## Protect `main`

Enable a protected branch rule (or an equivalent GitHub ruleset) that:

- requires a pull request before merging;
- requires one approving review, including review from a code owner once
  `CODEOWNERS` is introduced;
- dismisses stale approvals when new commits are pushed;
- requires all required status checks to pass and be up to date;
- requires conversation resolution before merge;
- blocks force pushes and branch deletion; and
- applies the rule to administrators.

Initial required checks, once Phase 1 exists, are Buf formatting/lint/breaking,
Bazel builds and tests, Go tests, `go vet`, `gofmt`, FDS smoke checks, and
clean-lock checks.

## Security and release controls

- Enable GitHub private vulnerability reporting.
- Configure a protected GitHub Actions `release` environment for publication,
  with maintainer approval and an environment-scoped `BUF_TOKEN` secret that
  can publish `buf.build/eshenhu/someip_proto`.
- Restrict release permissions to maintainers and use immutable tags, beginning
  with pre-1 SemVer `v0.1.0`.
- Require DCO sign-off and one maintainer review for every merged contribution.

The tag-triggered workflow is
[`release.yml`](../.github/workflows/release.yml). It requires the protected
environment and BSR token above; workflow files cannot configure those remote
settings themselves.

Record the completion date and the administrator who applied these settings in
the first public-release pull request.
