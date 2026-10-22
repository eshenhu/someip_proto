package semantic

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestCatalogedRules_matchRuleCatalog(t *testing.T) {
	catalog := readRuleCatalog(t)
	registered := make(map[string]rule, len(catalogedRules))
	for _, rule := range catalogedRules {
		if rule.code == "" || rule.reference == "" || rule.remediation == "" {
			t.Fatalf("cataloged rule %#v is incomplete", rule)
		}
		if _, found := registered[rule.code]; found {
			t.Fatalf("cataloged rule %q is registered more than once", rule.code)
		}
		registered[rule.code] = rule
		if !contains(catalog, rule.code) {
			t.Fatalf("docs/rule-catalog.md does not contain registered rule %q", rule.code)
		}
		if !catalogContainsAuthorityReference(catalog, rule.reference) {
			t.Fatalf("docs/rule-catalog.md does not contain authority reference %q for rule %q", rule.reference, rule.code)
		}
	}

	for _, code := range regexp.MustCompile(`SIP-[A-Z]+-[0-9]{3}`).FindAllString(catalog, -1) {
		if _, found := registered[code]; !found {
			t.Fatalf("docs/rule-catalog.md contains unregistered stable rule %q", code)
		}
	}
}

func readRuleCatalog(t *testing.T) string {
	t.Helper()
	candidates := []string{
		filepath.Join("..", "..", "docs", "rule-catalog.md"),
	}
	if runfiles := os.Getenv("RUNFILES_DIR"); runfiles != "" {
		if workspace := os.Getenv("TEST_WORKSPACE"); workspace != "" {
			candidates = append(candidates, filepath.Join(runfiles, workspace, "docs", "rule-catalog.md"))
		}
	}
	for _, candidate := range candidates {
		contents, err := os.ReadFile(candidate)
		if err == nil {
			return string(contents)
		}
	}
	t.Fatalf("read docs/rule-catalog.md from %v", candidates)
	return ""
}

func contains(text, value string) bool {
	return strings.Contains(text, value)
}

func catalogContainsAuthorityReference(catalog, reference string) bool {
	if contains(catalog, reference) {
		return true
	}
	separator := strings.LastIndexByte(reference, '#')
	return separator >= 0 && contains(catalog, reference[separator:])
}
