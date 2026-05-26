package config

import (
	"os"
	"testing"
)

func TestValidateConfigRequiresExplicitAdminPasswords(t *testing.T) {
	t.Setenv("GIN_MODE", "")

	AppConfig = Config{
		SecretKey:                "0123456789abcdef0123456789abcdef",
		MysqlPassword:            "db-password",
		MysqlPort:                3306,
		AdminUsername:            "admin",
		AdminPassword:            "",
		AuditAdminUsername:       "auditadmin",
		AuditAdminPassword:       "",
		AccessTokenExpireMinutes: 30,
	}

	err := validateConfig()
	if err == nil || err.Error() != "ADMIN_PASSWORD is required" {
		t.Fatalf("expected ADMIN_PASSWORD required error, got %v", err)
	}
}

func TestValidateConfigRejectsWeakAuditAdminPasswordInProduction(t *testing.T) {
	t.Setenv("GIN_MODE", "release")

	AppConfig = Config{
		SecretKey:                "0123456789abcdef0123456789abcdef",
		MysqlPassword:            "db-password",
		MysqlPort:                3306,
		AdminUsername:            "admin",
		AdminPassword:            "StrongAdminPassword#2026",
		AuditAdminUsername:       "auditadmin",
		AuditAdminPassword:       "audit123",
		AccessTokenExpireMinutes: 30,
	}

	err := validateConfig()
	if err == nil || err.Error() != "FATAL: Using default password 'audit123' is not allowed in production" {
		t.Fatalf("expected weak password rejection, got %v", err)
	}
}

func TestLoadConfigDoesNotBackfillWeakAdminDefaults(t *testing.T) {
	previousConfig := AppConfig
	t.Cleanup(func() {
		AppConfig = previousConfig
	})

	tempDir := t.TempDir()
	previousWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("chdir temp dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(previousWD)
	})

	t.Setenv("SECRET_KEY", "0123456789abcdef0123456789abcdef")
	t.Setenv("MYSQL_PASSWORD", "db-password")
	t.Setenv("ADMIN_PASSWORD", "StrongAdminPassword#2026")
	t.Setenv("AUDIT_ADMIN_PASSWORD", "StrongAuditPassword#2026")
	t.Setenv("GIN_MODE", "")

	if err := LoadConfig(); err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}
	if AppConfig.AdminPassword == "admin123" {
		t.Fatalf("LoadConfig must not populate legacy default ADMIN_PASSWORD")
	}
	if AppConfig.AuditAdminPassword == "audit123" {
		t.Fatalf("LoadConfig must not populate legacy default AUDIT_ADMIN_PASSWORD")
	}
}
