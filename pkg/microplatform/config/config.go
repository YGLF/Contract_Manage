package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type ServiceConfig struct {
	ServiceName string
	HTTPPort    int
	Environment string
	JWTSecret   string
	DBEnabled   bool
	DBDriver    string
	DBHost      string
	DBPort      int
	DBUser      string
	DBPassword  string
	DBName      string
	DBSchema    string
	UploadDir   string
}

func Load(serviceName string, defaultPort int) (ServiceConfig, error) {
	cfg := ServiceConfig{
		ServiceName: serviceName,
		HTTPPort:    envInt("SERVICE_PORT", defaultPort),
		Environment: envString("APP_ENV", "development"),
		JWTSecret:   envString("JWT_SECRET", envString("SECRET_KEY", "change-me-in-production")),
		DBEnabled:   envBool("DB_ENABLED", false),
		DBDriver:    envString("DB_DRIVER", envString("MYSQL_DRIVER", "mysql")),
		DBHost:      envString("DB_HOST", envString("MYSQL_HOST", "127.0.0.1")),
		DBPort:      envInt("DB_PORT", envInt("MYSQL_PORT", 3306)),
		DBUser:      envString("DB_USER", envString("MYSQL_USER", "root")),
		DBPassword:  envString("DB_PASSWORD", envString("MYSQL_PASSWORD", "")),
		DBName:      envString("DB_NAME", envString("MYSQL_DATABASE", "contract_manage")),
		DBSchema:    envString("DB_SCHEMA", strings.ReplaceAll(serviceName, "-", "_")),
		UploadDir:   envString("UPLOAD_DIR", "uploads"),
	}

	if strings.TrimSpace(cfg.ServiceName) == "" {
		return ServiceConfig{}, fmt.Errorf("service name is required")
	}

	return cfg, nil
}

func envString(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func envInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envBool(key string, fallback bool) bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if value == "" {
		return fallback
	}

	switch value {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}
