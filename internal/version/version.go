// Package version provides build-time provenance defaults for the CLI.
package version

// ToolVersion identifies the CLI build that emits an artifact.
// Release builds set this value with the Go linker.
var ToolVersion = "devel"

// SourceRevision identifies the source revision for a release build.
// Release builds set this value with the Go linker.
var SourceRevision string
