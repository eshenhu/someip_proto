package semantic

import (
	"fmt"
	"path"
	"strings"

	"github.com/eshenhu/someip_proto/internal/descriptor"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func (n *normalizer) validateArtifactProvenance() {
	provenance := n.manifest.GetProvenance()
	if provenance == nil {
		return
	}
	if !validProvenanceValue(provenance.GetToolVersion()) {
		n.diagnostics.error(ruleArtifact, descriptor.Location{}, "tool_version must not contain an absolute path, URI, or control character")
		provenance.ToolVersion = ""
	}
	if !validProvenanceValue(provenance.GetSourceRevision()) {
		n.diagnostics.error(ruleArtifact, descriptor.Location{}, "source_revision must not contain an absolute path, URI, or control character")
		provenance.SourceRevision = ""
	}
}

func validProvenanceValue(value string) bool {
	if value == "" {
		return true
	}
	if path.IsAbs(value) || strings.HasPrefix(value, "~") || strings.Contains(value, "\\") || strings.Contains(value, "://") {
		return false
	}
	if len(value) >= 3 && isASCIILetter(value[0]) && value[1] == ':' && value[2] == '/' {
		return false
	}
	for _, character := range value {
		if character < 0x20 || character == 0x7f {
			return false
		}
	}
	cleaned := path.Clean(value)
	return cleaned != "." && cleaned != ".." && !strings.HasPrefix(cleaned, "../")
}

func isASCIILetter(value byte) bool {
	return (value >= 'a' && value <= 'z') || (value >= 'A' && value <= 'Z')
}

func (n *normalizer) validateAnnotatedOrigin(value protoreflect.Descriptor, subject string) {
	if n == nil || n.set == nil || value == nil {
		return
	}
	file := value.ParentFile()
	if file == nil {
		return
	}
	origin := n.set.Origin(file)
	if origin == descriptor.FileOriginLocal || origin == descriptor.FileOriginLockedDependency {
		return
	}
	importPath := string(file.Path())
	if n.reportedUnapprovedOrigins == nil {
		n.reportedUnapprovedOrigins = make(map[string]struct{})
	}
	if _, reported := n.reportedUnapprovedOrigins[importPath]; reported {
		return
	}
	n.reportedUnapprovedOrigins[importPath] = struct{}{}
	n.diagnostics.error(
		ruleArtifact,
		n.set.Locations.Descriptor(value),
		fmt.Sprintf("%s %s is declared in %q with %s provenance; use a local file or an immutable buf.lock dependency", subject, value.FullName(), importPath, origin),
	)
}
