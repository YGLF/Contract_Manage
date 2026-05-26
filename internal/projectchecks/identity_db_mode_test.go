package projectchecks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIdentityDBModeDoesNotInitializeEmbeddedUsers(t *testing.T) {
	path := filepath.Clean(filepath.Join("..", "..", "internal", "microservices", "identity", "service.go"))
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	text := string(content)
	if strings.Contains(text, "service := New(jwtSecret)\n\tservice.db = db") ||
		strings.Contains(text, "service := New(jwtSecret)\r\n\tservice.db = db") {
		t.Fatalf("NewWithDB in %s must not seed embedded demo users when DB mode is enabled", path)
	}
	if !strings.Contains(text, "if s.db != nil") {
		t.Fatalf("login in %s must branch to database-backed authentication when DB mode is enabled", path)
	}
}
