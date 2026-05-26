package projectchecks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOfflineBundleIncludesProductionImageArtifacts(t *testing.T) {
	path := filepath.Clean(filepath.Join("..", "..", "scripts", "prepare-offline-bundle.sh"))
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	text := string(content)
	for _, required := range []string{
		"Dockerfile.microservice",
		"docker-compose.production.yml",
		"docker-compose.microservices.yml",
		".env.microservices.example",
		"APP_VERSION",
		"docker compose --env-file .env.microservices.example -f docker-compose.production.yml build",
		"anxin-contract/",
		"docker save",
		"anxin_contract_services_",
		"created_build_env",
		"rm -f \"${PROJECT_ROOT}/.env\"",
		"docker compose --env-file .env -f docker-compose.production.yml up -d",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("%s must package %s for offline delivery", path, required)
		}
	}
}

func TestBootstrapRuntimeRequiresProductionGatewayImage(t *testing.T) {
	path := filepath.Clean(filepath.Join("..", "..", "scripts", "bootstrap-runtime.sh"))
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	text := string(content)
	for _, required := range []string{
		"APP_VERSION",
		"anxin-contract/gateway-service:${APP_VERSION}",
		"生产 compose 无法在离线服务器直接启动",
		"docker compose --env-file .env -f docker-compose.production.yml up -d",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("%s must contain %q to validate offline production image readiness", path, required)
		}
	}
}
