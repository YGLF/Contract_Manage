package projectchecks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigDoesNotShipDefaultAdminPasswords(t *testing.T) {
	path := filepath.Clean(filepath.Join("..", "..", "config", "config.go"))
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	text := string(content)
	disallowed := []string{
		`viper.SetDefault("ADMIN_PASSWORD", "admin123")`,
		`viper.SetDefault("AUDIT_ADMIN_PASSWORD", "audit123")`,
	}

	for _, snippet := range disallowed {
		if strings.Contains(text, snippet) {
			t.Fatalf("%s must not define weak default credential %s", path, snippet)
		}
	}
}
