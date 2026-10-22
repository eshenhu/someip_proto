package descriptor

import (
	"crypto/sha256"
	"fmt"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

const (
	projectImportPrefix = "io/github/someip_proto/"
	builtinImportPrefix = "google/protobuf/"

	// lockedBufLockSHA256 is generated from proto/buf.lock. The lock-check
	// script rejects drift so FDS parsing never needs filesystem access.
	lockedBufLockSHA256 = "4e4eb8118feed0b23ba70f7738e78d2fce8b00c30422fb04cc4acfebe851528d"
)

// FileOrigin identifies the trusted origin classification of an FDS file.
type FileOrigin string

const (
	// FileOriginUnknown identifies a file that is neither local, built in, nor
	// recorded in the immutable Buf dependency ledger.
	FileOriginUnknown FileOrigin = "unknown"
	// FileOriginLocal identifies a file in this repository's public import
	// namespace.
	FileOriginLocal FileOrigin = "local"
	// FileOriginLockedDependency identifies a descriptor whose exact bytes are
	// recorded by the immutable Buf dependency ledger.
	FileOriginLockedDependency FileOrigin = "locked_dependency"
	// FileOriginBuiltin identifies a Protobuf well-known-type descriptor.
	FileOriginBuiltin FileOrigin = "builtin"
)

type lockedDependencyFile struct {
	sha256 string
	module string
	commit string
	digest string
}

// lockedDependencyFiles is a checked-in ledger generated from the immutable Buf
// dependency closure. Each entry binds a logical import path to deterministic
// descriptor bytes and its reviewed Buf module, commit, and digest. The current
// buf.lock has no dependencies, so external annotated services and payloads are
// deliberately rejected until a lock update adds matching reviewed entries.
//
// The loader deliberately consults only this compiled ledger: it never reads
// buf.lock or contacts a registry while parsing an FDS.
var lockedDependencyFiles = map[string]lockedDependencyFile{}

func classifyFileOrigins(set *descriptorpb.FileDescriptorSet) map[string]FileOrigin {
	origins := make(map[string]FileOrigin, len(set.GetFile()))
	for _, file := range set.GetFile() {
		importPath := file.GetName()
		switch {
		case strings.HasPrefix(importPath, projectImportPrefix):
			origins[importPath] = FileOriginLocal
		case strings.HasPrefix(importPath, builtinImportPrefix):
			origins[importPath] = FileOriginBuiltin
		case lockedDependencyFileMatches(importPath, file):
			origins[importPath] = FileOriginLockedDependency
		default:
			origins[importPath] = FileOriginUnknown
		}
	}
	return origins
}

func lockedDependencyFileMatches(importPath string, file *descriptorpb.FileDescriptorProto) bool {
	entry, found := lockedDependencyFiles[importPath]
	if !found || entry.sha256 == "" || entry.module == "" || entry.commit == "" || entry.digest == "" {
		return false
	}
	encoded, err := proto.MarshalOptions{Deterministic: true}.Marshal(file)
	if err != nil {
		return false
	}
	sum := sha256.Sum256(encoded)
	return fmt.Sprintf("%x", sum) == entry.sha256
}
