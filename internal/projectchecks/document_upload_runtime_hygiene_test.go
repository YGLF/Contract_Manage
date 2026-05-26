package projectchecks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProductionDocumentUploadsUseAppOwnedRuntimePath(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
	composePath := filepath.Join(root, "docker-compose.production.yml")
	dockerfilePath := filepath.Join(root, "Dockerfile.microservice")

	composeBytes, err := os.ReadFile(composePath)
	if err != nil {
		t.Fatalf("read %s: %v", composePath, err)
	}
	dockerfileBytes, err := os.ReadFile(dockerfilePath)
	if err != nil {
		t.Fatalf("read %s: %v", dockerfilePath, err)
	}

	compose := strings.ReplaceAll(string(composeBytes), "\r\n", "\n")
	dockerfile := string(dockerfileBytes)

	required := []string{
		"UPLOAD_DIR: ${UPLOAD_DIR:-uploads}",
		"- uploads:/app/uploads",
		"WORKDIR /app",
	}
	for _, snippet := range required {
		if !strings.Contains(compose+"\n"+dockerfile, snippet) {
			t.Fatalf("production document upload runtime must contain %q", snippet)
		}
	}

	disallowed := []string{
		"UPLOAD_DIR: ${UPLOAD_DIR:-/workspace/uploads}",
		"- uploads:/workspace/uploads",
	}
	for _, snippet := range disallowed {
		if strings.Contains(compose, snippet) {
			t.Fatalf("%s must not contain legacy development upload path %q", composePath, snippet)
		}
	}
}
