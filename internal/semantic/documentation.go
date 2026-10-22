package semantic

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/eshenhu/someip_proto/internal/descriptor"
)

func (n *normalizer) validateDocumentationURIs(location descriptor.Location, subject string, values []string) {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if _, duplicate := seen[value]; duplicate {
			n.diagnostics.error(ruleArtifact, location, fmt.Sprintf("%s repeats documentation URI %q", subject, value))
			continue
		}
		seen[value] = struct{}{}
		parsed, err := url.ParseRequestURI(value)
		if err != nil || !allowedDocumentationURI(parsed) {
			n.diagnostics.error(ruleArtifact, location, fmt.Sprintf("%s documentation URI %q must be an absolute http, https, or mailto URI", subject, value))
		}
	}
}

func allowedDocumentationURI(value *url.URL) bool {
	if value == nil {
		return false
	}
	switch strings.ToLower(value.Scheme) {
	case "http", "https":
		return value.Host != ""
	case "mailto":
		return value.Opaque != ""
	default:
		return false
	}
}
