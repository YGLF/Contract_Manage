package projectchecks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const operationalScriptTag = "//go:build operational_scripts"

func TestOperationalScriptsRequireExplicitBuildTagAndNoHardcodedRootDSN(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", "..", "scripts"))

	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read scripts directory: %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".go" {
			continue
		}

		path := filepath.Join(root, entry.Name())
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}

		text := string(content)
		if !strings.HasPrefix(text, operationalScriptTag+"\n") && !strings.HasPrefix(text, operationalScriptTag+"\r\n") {
			t.Errorf("%s must start with %q so normal builds/tests do not compile one-off operational tools", path, operationalScriptTag)
		}
		if strings.Contains(text, "root:root@tcp") {
			t.Errorf("%s contains a hardcoded privileged database DSN", path)
		}
	}
}
