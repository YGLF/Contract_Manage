package projectchecks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidationScriptsAllowProductionComposeOverride(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
	for _, file := range []string{
		filepath.Join(root, "scripts", "check-env.sh"),
		filepath.Join(root, "scripts", "health-check.sh"),
	} {
		content, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		text := string(content)
		for _, required := range []string{
			`COMPOSE_FILE="${COMPOSE_FILE:-${PROJECT_ROOT}/docker-compose.microservices.yml}"`,
			"docker-compose.production.yml",
		} {
			if !strings.Contains(text, required) {
				t.Fatalf("%s must contain %q so production compose validation can reuse the same script path", file, required)
			}
		}
	}
}

func TestValidationScriptsUseSafeEnvAndComposeDetection(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))

	checkEnvBytes, err := os.ReadFile(filepath.Join(root, "scripts", "check-env.sh"))
	if err != nil {
		t.Fatalf("read check-env.sh: %v", err)
	}
	healthBytes, err := os.ReadFile(filepath.Join(root, "scripts", "health-check.sh"))
	if err != nil {
		t.Fatalf("read health-check.sh: %v", err)
	}

	checkEnv := string(checkEnvBytes)
	health := string(healthBytes)

	for _, content := range []string{checkEnv, health} {
		if strings.Contains(content, "source \"${ENV_FILE}\"") {
			t.Fatal("validation scripts must not source .env directly; they should parse key/value pairs without executing shell")
		}
		for _, required := range []string{
			"docker-compose",
			"resolve_docker_compose",
			"load_env_file",
		} {
			if !strings.Contains(content, required) {
				t.Fatalf("validation scripts must contain %q", required)
			}
		}
	}

	for _, required := range []string{
		"timeout",
		"DOCKER_COMPOSE_LABEL",
	} {
		if !strings.Contains(checkEnv, required) {
			t.Fatalf("check-env.sh must contain %q", required)
		}
	}

	for _, required := range []string{
		"mktemp",
		"unhealthy",
		"Compose 状态查询失败",
	} {
		if !strings.Contains(health, required) {
			t.Fatalf("health-check.sh must contain %q", required)
		}
	}
}
