package projectchecks

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestProductionComposeUsesPrebuiltServiceImages(t *testing.T) {
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

	for _, forbidden := range []string{"go run", "./:/workspace", ".:/workspace"} {
		if strings.Contains(compose, forbidden) {
			t.Fatalf("%s must not contain %q; production runtime should use immutable service images", composePath, forbidden)
		}
	}
	for _, required := range []string{"Dockerfile.microservice", "restart: unless-stopped", "env_file:", "ENTRYPOINT [\"/app/service\"]", "ca-certificates", "wget"} {
		if !strings.Contains(compose+"\n"+dockerfile, required) {
			t.Fatalf("production image path must contain %q", required)
		}
	}
	if !strings.Contains(dockerfile, "USER appuser") {
		t.Fatalf("%s must run the service as a non-root user", dockerfilePath)
	}

	entries, err := os.ReadDir(filepath.Join(root, "cmd"))
	if err != nil {
		t.Fatalf("read cmd directory: %v", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		serviceName := entry.Name()
		if !strings.Contains(compose, serviceName+":") {
			t.Fatalf("%s must define service %q", composePath, serviceName)
		}
		if !strings.Contains(compose, "SERVICE_NAME: "+serviceName) {
			t.Fatalf("%s must build %q with SERVICE_NAME arg", composePath, serviceName)
		}
	}
}

func TestProductionComposeAddsHealthchecksForCriticalServices(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
	composePath := filepath.Join(root, "docker-compose.production.yml")

	composeBytes, err := os.ReadFile(composePath)
	if err != nil {
		t.Fatalf("read %s: %v", composePath, err)
	}

	compose := strings.ReplaceAll(string(composeBytes), "\r\n", "\n")
	for _, required := range []string{
		"x-http-healthcheck: &http-healthcheck\n",
		"healthcheck:\n",
		"CMD-SHELL",
		"/health",
	} {
		if !strings.Contains(compose, required) {
			t.Fatalf("%s must define shared production healthcheck snippet %q", composePath, required)
		}
	}

	for _, service := range []string{
		"gateway-service",
		"identity-service",
		"audit-service",
		"contract-service",
		"document-service",
		"approval-workflow-service",
		"notification-service",
		"report-service",
		"search-ai-service",
	} {
		re := regexp.MustCompile(`(?ms)^  ` + regexp.QuoteMeta(service) + `:\n(.*?)(?:^  [a-z0-9-]+:\n|^volumes:\n|\z)`)
		matches := re.FindStringSubmatch(compose)
		if len(matches) != 2 {
			t.Fatalf("%s must define service %q", composePath, service)
		}
		block := matches[0]
		if !strings.Contains(block, "*http-healthcheck") {
			t.Fatalf("%s must merge shared healthcheck into %q", composePath, service)
		}
	}
}

func TestProductionComposeTreatsOutboxDispatcherAsOperationalJob(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
	composePath := filepath.Join(root, "docker-compose.production.yml")

	composeBytes, err := os.ReadFile(composePath)
	if err != nil {
		t.Fatalf("read %s: %v", composePath, err)
	}

	compose := strings.ReplaceAll(string(composeBytes), "\r\n", "\n")
	re := regexp.MustCompile(`(?ms)^  outbox-dispatcher:\n(.*?)(?:^  [a-z0-9-]+:\n|^volumes:\n|\z)`)
	matches := re.FindStringSubmatch(compose)
	if len(matches) != 2 {
		t.Fatalf("%s must define service %q", composePath, "outbox-dispatcher")
	}
	block := matches[0]
	for _, required := range []string{
		"*job-defaults",
		`profiles: ["ops"]`,
		"restart: \"no\"",
	} {
		if !strings.Contains(compose, required) && !strings.Contains(block, required) {
			t.Fatalf("%s must treat outbox-dispatcher as an operational job with %q", composePath, required)
		}
	}
}
