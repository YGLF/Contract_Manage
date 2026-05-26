package projectchecks

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestDatabaseBootstrapDoesNotShipHardcodedCredentials(t *testing.T) {
	path := filepath.Clean(filepath.Join("..", "..", "init.sql"))
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	text := string(content)
	for _, rule := range []struct {
		name    string
		pattern string
	}{
		{name: "hardcoded CREATE USER password", pattern: `(?i)IDENTIFIED\s+BY\s+['"]`},
		{name: "overbroad database grant", pattern: `(?i)GRANT\s+ALL\s+PRIVILEGES`},
	} {
		if regexp.MustCompile(rule.pattern).FindStringIndex(text) != nil {
			t.Fatalf("%s must not contain %s", path, rule.name)
		}
	}
}
