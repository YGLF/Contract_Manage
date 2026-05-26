package db

import (
	"fmt"
	"strings"

	platformconfig "contract-manage/pkg/microplatform/config"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func Open(cfg platformconfig.ServiceConfig) (*gorm.DB, error) {
	driver := strings.TrimSpace(strings.ToLower(cfg.DBDriver))
	if driver == "" {
		driver = "mysql"
	}

	switch driver {
	case "mysql", "kingbase-mysql", "kingbase_mysql":
		dsn := fmt.Sprintf(
			"%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			cfg.DBUser,
			cfg.DBPassword,
			cfg.DBHost,
			cfg.DBPort,
			cfg.DBName,
		)
		return gorm.Open(mysql.Open(dsn), &gorm.Config{})
	default:
		return nil, fmt.Errorf("unsupported db driver: %s", cfg.DBDriver)
	}
}
