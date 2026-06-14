package config

import (
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	AppEnv               string
	HTTPAddr             string
	DatabaseDSN          string
	JWTSecret            string
	JWTExpireHours       int
	AdminDefaultUsername string
	AdminDefaultPassword string
	SMSPoolAPIKey        string
	SMSPoolBaseURL       string
	SMSPoolTimeout       time.Duration
	OrderPollInterval    time.Duration
	OrderTimeout         time.Duration
	CardExportDir        string
}

func Load() Config {
	_ = godotenv.Load()

	return Config{
		AppEnv:               getEnv("APP_ENV", "development"),
		HTTPAddr:             getEnv("HTTP_ADDR", ":8080"),
		DatabaseDSN:          getEnv("DATABASE_DSN", "host=127.0.0.1 user=postgres password=postgres dbname=sms_middle_platform port=5432 sslmode=disable TimeZone=Asia/Shanghai"),
		JWTSecret:            getEnv("JWT_SECRET", "change-me"),
		JWTExpireHours:       getEnvInt("JWT_EXPIRE_HOURS", 24),
		AdminDefaultUsername: getEnv("ADMIN_DEFAULT_USERNAME", "admin"),
		AdminDefaultPassword: getEnv("ADMIN_DEFAULT_PASSWORD", "admin123456"),
		SMSPoolAPIKey:        getEnv("SMSPOOL_API_KEY", ""),
		SMSPoolBaseURL:       getEnv("SMSPOOL_BASE_URL", "https://api.smspool.net"),
		SMSPoolTimeout:       time.Duration(getEnvInt("SMSPOOL_TIMEOUT_SECONDS", 15)) * time.Second,
		OrderPollInterval:    time.Duration(getEnvInt("ORDER_POLL_INTERVAL_SECONDS", 8)) * time.Second,
		OrderTimeout:         time.Duration(getEnvInt("ORDER_TIMEOUT_SECONDS", 1200)) * time.Second,
		CardExportDir:        getEnv("CARD_EXPORT_DIR", "storage/card_exports"),
	}
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func getEnvInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
