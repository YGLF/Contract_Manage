package projectchecks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMicroserviceComposeIncludesRestartPolicyAndGatewayHealthcheck(t *testing.T) {
	path := filepath.Clean(filepath.Join("..", "..", "docker-compose.microservices.yml"))
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	text := strings.ReplaceAll(string(content), "\r\n", "\n")
	requiredSnippets := []string{
		"x-go-service-defaults: &go-service-defaults\n",
		"  restart: unless-stopped\n",
		"gateway-service:\n",
		"    <<: *go-service-defaults\n",
		"    healthcheck:\n",
		"      test:",
		"/health",
	}

	for _, snippet := range requiredSnippets {
		if !strings.Contains(text, snippet) {
			t.Fatalf("%s must contain %q to support production restarts and health probes", path, snippet)
		}
	}
}
