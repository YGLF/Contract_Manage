package projectchecks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDockerIgnoreExcludesLocalSecretsAndBuildArtifacts(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
	path := filepath.Join(root, ".dockerignore")

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	entries := parseDockerIgnoreEntries(string(content))
	required := []string{
		".env",
		".codex",
		".gocache",
		"_legacy_monolith_archive",
		"offline-bundle",
		"*.exe",
		"*.db",
		"*.sqlite",
		"uploads",
		"frontend/node_modules",
		"frontend/dist",
	}

	for _, pattern := range required {
		if !entries[pattern] {
			t.Errorf("%s must exclude %q from Docker build contexts", path, pattern)
		}
	}
}

func parseDockerIgnoreEntries(content string) map[string]bool {
	entries := make(map[string]bool)
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		entries[strings.TrimSuffix(line, "/")] = true
	}
	return entries
}
