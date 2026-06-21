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
	DataEncryptionKey    string
	JWTExpireHours       int
	AdminDefaultUsername string
	AdminDefaultPassword string
	SMSPoolAPIKey        string
	SMSPoolBaseURL       string
	SMSPoolTimeout       time.Duration
	FirefoxAPIKey        string
	FirefoxBaseURL       string
	FirefoxTimeout       time.Duration
	HeroSMSAPIKey        string
	HeroSMSBaseURL       string
	HeroSMSTimeout       time.Duration
	SMSBowerAPIKey       string
	SMSBowerBaseURL      string
	SMSBowerTimeout      time.Duration
	LubanSMSAPIKey       string
	LubanSMSBaseURL      string
	LubanSMSTimeout      time.Duration
	SMS68APIKey          string
	SMS68BaseURL         string
	SMS68Timeout         time.Duration
	SMS68MetadataToken   string
	SMS62USAPIKey        string
	SMS62USBaseURL       string
	SMS62USTimeout       time.Duration
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
		DataEncryptionKey:    getEnv("DATA_ENCRYPTION_KEY", getEnv("JWT_SECRET", "change-me")),
		JWTExpireHours:       getEnvInt("JWT_EXPIRE_HOURS", 24),
		AdminDefaultUsername: getEnv("ADMIN_DEFAULT_USERNAME", "admin"),
		AdminDefaultPassword: getEnv("ADMIN_DEFAULT_PASSWORD", "admin123456"),
		SMSPoolAPIKey:        getEnv("SMSPOOL_API_KEY", ""),
		SMSPoolBaseURL:       getEnv("SMSPOOL_BASE_URL", "https://api.smspool.net"),
		SMSPoolTimeout:       time.Duration(getEnvInt("SMSPOOL_TIMEOUT_SECONDS", 15)) * time.Second,
		FirefoxAPIKey:        getEnv("FIREFOX_API_KEY", ""),
		FirefoxBaseURL:       getEnv("FIREFOX_BASE_URL", "http://www.firefox.fun"),
		FirefoxTimeout:       time.Duration(getEnvInt("FIREFOX_TIMEOUT_SECONDS", 15)) * time.Second,
		HeroSMSAPIKey:        getEnv("HEROSMS_API_KEY", ""),
		HeroSMSBaseURL:       getEnv("HEROSMS_BASE_URL", "https://hero-sms.com"),
		HeroSMSTimeout:       time.Duration(getEnvInt("HEROSMS_TIMEOUT_SECONDS", 15)) * time.Second,
		SMSBowerAPIKey:       getEnv("SMSBOWER_API_KEY", ""),
		SMSBowerBaseURL:      getEnv("SMSBOWER_BASE_URL", "https://smsbower.page"),
		SMSBowerTimeout:      time.Duration(getEnvInt("SMSBOWER_TIMEOUT_SECONDS", 15)) * time.Second,
		LubanSMSAPIKey:       getEnv("LUBANSMS_API_KEY", ""),
		LubanSMSBaseURL:      getEnv("LUBANSMS_BASE_URL", "https://lubansms.com"),
		LubanSMSTimeout:      time.Duration(getEnvInt("LUBANSMS_TIMEOUT_SECONDS", 15)) * time.Second,
		SMS68APIKey:          getEnv("SMS68_API_KEY", ""),
		SMS68BaseURL:         getEnv("SMS68_BASE_URL", "https://api.68sms.com"),
		SMS68Timeout:         time.Duration(getEnvInt("SMS68_TIMEOUT_SECONDS", 15)) * time.Second,
		SMS68MetadataToken:   getEnv("SMS68_METADATA_TOKEN", ""),
		SMS62USAPIKey:        getEnv("SMS62US_API_KEY", ""),
		SMS62USBaseURL:       getEnv("SMS62US_BASE_URL", "https://api.62-us.com"),
		SMS62USTimeout:       time.Duration(getEnvInt("SMS62US_TIMEOUT_SECONDS", 15)) * time.Second,
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
