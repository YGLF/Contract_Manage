package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

// Config 应用配置结构体
// 用于存储从环境变量或配置文件加载的所有配置项
type Config struct {
	AppName    string `mapstructure:"APP_NAME"`    // 应用名称
	AppVersion string `mapstructure:"APP_VERSION"` // 应用版本号

	// MySQL数据库连接配置
	MysqlHost     string `mapstructure:"MYSQL_HOST"`     // 数据库主机地址
	MysqlPort     int    `mapstructure:"MYSQL_PORT"`     // 数据库端口号
	MysqlUser     string `mapstructure:"MYSQL_USER"`     // 数据库用户名
	MysqlPassword string `mapstructure:"MYSQL_PASSWORD"` // 数据库密码
	MysqlDatabase string `mapstructure:"MYSQL_DATABASE"` // 数据库名称

	// JWT认证配置
	SecretKey                string `mapstructure:"SECRET_KEY"`                  // JWT签名密钥，用于生成和验证token
	JwtAlgorithm             string `mapstructure:"JWT_ALGORITHM"`               // JWT签名算法，默认HS256
	AccessTokenExpireMinutes int    `mapstructure:"ACCESS_TOKEN_EXPIRE_MINUTES"` // Access Token过期时间（分钟）

	// 文件上传配置
	UploadDir string `mapstructure:"UPLOAD_DIR"` // 文件上传目录

	// 超级管理员账户配置
	AdminUsername string `mapstructure:"ADMIN_USERNAME"` // 超级管理员用户名
	AdminPassword string `mapstructure:"ADMIN_PASSWORD"` // 超级管理员密码
	AdminEmail    string `mapstructure:"ADMIN_EMAIL"`    // 超级管理员邮箱

	// 审计管理员账户配置
	AuditAdminUsername string `mapstructure:"AUDIT_ADMIN_USERNAME"` // 审计管理员用户名
	AuditAdminPassword string `mapstructure:"AUDIT_ADMIN_PASSWORD"` // 审计管理员密码
	AuditAdminEmail    string `mapstructure:"AUDIT_ADMIN_EMAIL"`    // 审计管理员邮箱

	// 密码机/HSM配置
	HSMEndpoint string `mapstructure:"HSM_ENDPOINT"` // 密码机服务地址
	HSMAppID    string `mapstructure:"HSM_APP_ID"`   // 密码机应用ID
	HSMEnabled  bool   `mapstructure:"HSM_ENABLED"`  // 是否启用密码机
	SM4Key      string `mapstructure:"SM4_KEY"`      // SM4对称密钥（AES密钥）
	SM4Enabled  bool   `mapstructure:"SM4_ENABLED"`  // 是否启用SM4加密
	AESKey      string `mapstructure:"AES_KEY"`      // AES对称密钥
	AESEnabled  bool   `mapstructure:"AES_ENABLED"`  // 是否启用AES加密
}

// AppConfig 全局配置实例
// 在应用启动时通过LoadConfig()加载并填充
var AppConfig Config

// LoadConfig 加载配置文件
// 优先级：环境变量 > .env文件 > 默认值
// 1. 首先设置默认配置值
// 2. 读取.env配置文件
// 3. 允许环境变量覆盖配置文件的值
// 4. 验证配置的有效性
func LoadConfig() error {
	viper.Reset()
	// 设置配置文件路径为当前目录的.env文件
	viper.SetConfigFile(".env")
	// 允许环境变量自动覆盖配置文件中的值
	viper.AutomaticEnv()

	// 设置默认配置值
	viper.SetDefault("APP_NAME", "安信合同管理系统")
	viper.SetDefault("APP_VERSION", "1.0.0")
	viper.SetDefault("MYSQL_HOST", "localhost")
	viper.SetDefault("MYSQL_PORT", 3306)
	viper.SetDefault("MYSQL_USER", "root")
	viper.SetDefault("MYSQL_PASSWORD", "rootroots")
	viper.SetDefault("MYSQL_DATABASE", "contract_manage")
	viper.SetDefault("JWT_ALGORITHM", "HS256")
	viper.SetDefault("ACCESS_TOKEN_EXPIRE_MINUTES", 30)
	viper.SetDefault("UPLOAD_DIR", "uploads")

	// 默认超级管理员账户
	viper.SetDefault("ADMIN_USERNAME", "admin")
	viper.SetDefault("ADMIN_EMAIL", "admin@example.com")

	// 默认审计管理员账户
	viper.SetDefault("AUDIT_ADMIN_USERNAME", "auditadmin")
	viper.SetDefault("AUDIT_ADMIN_EMAIL", "audit@example.com")

	// 密码机默认配置
	viper.SetDefault("HSM_ENABLED", false)
	viper.SetDefault("SM4_ENABLED", false)
	viper.SetDefault("AES_ENABLED", false)
	bindConfigEnv()

	// 读取配置文件，如果文件不存在也不报错（因为环境变量可能已提供所有配置）
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok && !os.IsNotExist(err) {
			return err
		}
	}

	// 将配置解析到AppConfig结构体
	if err := viper.Unmarshal(&AppConfig); err != nil {
		return err
	}

	// 验证配置的有效性
	if err := validateConfig(); err != nil {
		return err
	}

	return nil
}

func bindConfigEnv() {
	for _, key := range []string{
		"APP_NAME",
		"APP_VERSION",
		"MYSQL_HOST",
		"MYSQL_PORT",
		"MYSQL_USER",
		"MYSQL_PASSWORD",
		"MYSQL_DATABASE",
		"SECRET_KEY",
		"JWT_ALGORITHM",
		"ACCESS_TOKEN_EXPIRE_MINUTES",
		"UPLOAD_DIR",
		"ADMIN_USERNAME",
		"ADMIN_PASSWORD",
		"ADMIN_EMAIL",
		"AUDIT_ADMIN_USERNAME",
		"AUDIT_ADMIN_PASSWORD",
		"AUDIT_ADMIN_EMAIL",
		"HSM_ENDPOINT",
		"HSM_APP_ID",
		"HSM_ENABLED",
		"SM4_KEY",
		"SM4_ENABLED",
		"AES_KEY",
		"AES_ENABLED",
	} {
		_ = viper.BindEnv(key)
	}
}

// validateConfig 验证配置项的有效性
// 检查必填字段是否存在，值是否合法
func validateConfig() error {
	// 检查JWT签名密钥是否已配置
	if AppConfig.SecretKey == "" {
		return fmt.Errorf("SECRET_KEY is required")
	}

	// 检查密钥长度，至少32位以保证安全性
	if len(AppConfig.SecretKey) < 32 {
		return fmt.Errorf("SECRET_KEY must be at least 32 characters")
	}

	// 安全检查：生产环境禁止使用默认/弱密钥
	isProduction := os.Getenv("GIN_MODE") == "release"
	if AppConfig.SecretKey == "your-secret-key-change-in-production" {
		if isProduction {
			return fmt.Errorf("FATAL: Using default SECRET_KEY is not allowed in production")
		}
		fmt.Println("WARNING: Using default SECRET_KEY. Please change it in production!")
	}

	// 检查数据库密码是否已配置
	if AppConfig.MysqlPassword == "" {
		return fmt.Errorf("MYSQL_PASSWORD is required")
	}

	// 如果未指定主机，默认使用localhost
	if AppConfig.MysqlHost == "" {
		AppConfig.MysqlHost = "localhost"
	}

	// 验证端口号范围
	if AppConfig.MysqlPort < 1 || AppConfig.MysqlPort > 65535 {
		return fmt.Errorf("MYSQL_PORT must be between 1 and 65535")
	}

	// 限制Token过期时间范围：最短5分钟，最长480分钟（8小时工作制）
	if AppConfig.AccessTokenExpireMinutes < 5 {
		AppConfig.AccessTokenExpireMinutes = 5
	}
	if AppConfig.AccessTokenExpireMinutes > 480 {
		AppConfig.AccessTokenExpireMinutes = 480
	}

	// 去除字符串配置的首尾空格
	AppConfig.MysqlDatabase = strings.TrimSpace(AppConfig.MysqlDatabase)
	AppConfig.MysqlUser = strings.TrimSpace(AppConfig.MysqlUser)
	AppConfig.AdminUsername = strings.TrimSpace(AppConfig.AdminUsername)
	AppConfig.AdminPassword = strings.TrimSpace(AppConfig.AdminPassword)
	AppConfig.AuditAdminUsername = strings.TrimSpace(AppConfig.AuditAdminUsername)
	AppConfig.AuditAdminPassword = strings.TrimSpace(AppConfig.AuditAdminPassword)

	if AppConfig.AdminUsername == "" {
		return fmt.Errorf("ADMIN_USERNAME is required")
	}
	if AppConfig.AdminPassword == "" {
		return fmt.Errorf("ADMIN_PASSWORD is required")
	}
	if AppConfig.AuditAdminUsername == "" {
		return fmt.Errorf("AUDIT_ADMIN_USERNAME is required")
	}
	if AppConfig.AuditAdminPassword == "" {
		return fmt.Errorf("AUDIT_ADMIN_PASSWORD is required")
	}

	// 生产环境禁止使用默认管理员密码
	defaultPasswords := []string{"admin123", "audit123", "admin@123456", "password", "123456"}
	for _, pwd := range defaultPasswords {
		if AppConfig.AdminPassword == pwd || AppConfig.AuditAdminPassword == pwd {
			if isProduction {
				return fmt.Errorf("FATAL: Using default password '%s' is not allowed in production", pwd)
			}
			fmt.Printf("WARNING: Using default admin password. Please change it in production!\n")
			break
		}
	}

	return nil
}
