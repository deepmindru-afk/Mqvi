package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Server          ServerConfig
	Database        DatabaseConfig
	JWT             JWTConfig
	LiveKit         LiveKitConfig
	Upload          UploadConfig
	Antivirus       AntivirusConfig
	FileRateLimit   FileRateLimitConfig
	Email           EmailConfig
	Klipy           KlipyConfig
	EncryptionKey   string // AES-256 key (64 hex chars = 32 bytes) for LiveKit credential encryption
	HetznerAPIToken string // Hetzner Cloud API token (read-only) — optional
}

// EmailConfig — optional. If RESEND_API_KEY is empty, password reset is disabled.
type EmailConfig struct {
	ResendAPIKey string
	FromEmail    string // e.g. noreply@mqvi.app
	AppURL       string // e.g. https://app.mqvi.app — used in reset links
}

// KlipyConfig — optional. If KLIPY_API_KEY is empty, GIF search is disabled.
type KlipyConfig struct {
	APIKey string
}

type ServerConfig struct {
	Host string
	Port int
}

type DatabaseConfig struct {
	Path string
}

type JWTConfig struct {
	Secret             string
	AccessTokenExpiry  int // minutes (default: 15)
	RefreshTokenExpiry int // days (default: 7)
}

type LiveKitConfig struct {
	URL       string
	APIKey    string
	APISecret string
}

type UploadConfig struct {
	Dir               string
	MaxSize           int64 // bytes (default: 500MB)
	DefaultQuotaBytes int64 // per-user storage quota (default: 10GB)
	// PublicURL is the absolute base URL prepended to file URLs when they need
	// to be served cross-origin or via CDN, e.g. "https://files.mqvi.net".
	// Empty means file URLs are returned as relative paths (current behaviour).
	PublicURL string
	// SignedURLSecret is the HMAC-SHA256 key for signing file URLs (32 bytes, base64-encoded).
	// Required in production. If empty, server refuses to start.
	SignedURLSecret string
	// SignedURLSecretPrev is the previous signing key, still accepted for verification
	// during key rotation. URLs are only signed with the active key.
	SignedURLSecretPrev string
}

type AntivirusConfig struct {
	Enabled                 bool
	ClamAVAddr              string
	TimeoutSeconds          int
	MaxScanSizeBytes        int64
	UnavailablePolicy       string
	TooLargePolicy          string
	CleanCacheTTLHours      int
	InfectedCacheTTLDays    int
	CircuitFailureThreshold int
	CircuitWindowSeconds    int
	CircuitOpenSeconds      int
}

type FileRateLimitConfig struct {
	UserPerMin int
	IPPerMin   int
}

// Load reads configuration from environment variables.
// Falls back to .env file in development.
func Load() (*Config, error) {
	_ = godotenv.Load()

	port, err := strconv.Atoi(getEnv("SERVER_PORT", "9090"))
	if err != nil {
		return nil, fmt.Errorf("invalid SERVER_PORT: %w", err)
	}

	accessExpiry, err := strconv.Atoi(getEnv("JWT_ACCESS_EXPIRY_MINUTES", "15"))
	if err != nil {
		return nil, fmt.Errorf("invalid JWT_ACCESS_EXPIRY_MINUTES: %w", err)
	}

	refreshExpiry, err := strconv.Atoi(getEnv("JWT_REFRESH_EXPIRY_DAYS", "7"))
	if err != nil {
		return nil, fmt.Errorf("invalid JWT_REFRESH_EXPIRY_DAYS: %w", err)
	}

	maxSize, err := strconv.ParseInt(getEnv("UPLOAD_MAX_SIZE", "104857600"), 10, 64) // 100 MB — Cloudflare body limit
	if err != nil {
		return nil, fmt.Errorf("invalid UPLOAD_MAX_SIZE: %w", err)
	}

	defaultQuota, err := strconv.ParseInt(getEnv("MQVI_DEFAULT_QUOTA_BYTES", "10737418240"), 10, 64) // 10GB
	if err != nil {
		return nil, fmt.Errorf("invalid MQVI_DEFAULT_QUOTA_BYTES: %w", err)
	}

	fileRateUser, err := strconv.Atoi(getEnv("MQVI_FILE_RATE_USER_PER_MIN", "600"))
	if err != nil {
		return nil, fmt.Errorf("invalid MQVI_FILE_RATE_USER_PER_MIN: %w", err)
	}

	fileRateIP, err := strconv.Atoi(getEnv("MQVI_FILE_RATE_IP_PER_MIN", "2000"))
	if err != nil {
		return nil, fmt.Errorf("invalid MQVI_FILE_RATE_IP_PER_MIN: %w", err)
	}

	avEnabled, err := strconv.ParseBool(getEnv("MQVI_ANTIVIRUS_ENABLED", "true"))
	if err != nil {
		return nil, fmt.Errorf("invalid MQVI_ANTIVIRUS_ENABLED: %w", err)
	}
	avTimeout, err := strconv.Atoi(getEnv("MQVI_ANTIVIRUS_TIMEOUT_SECONDS", "10"))
	if err != nil {
		return nil, fmt.Errorf("invalid MQVI_ANTIVIRUS_TIMEOUT_SECONDS: %w", err)
	}
	avMaxScanMB, err := strconv.ParseInt(getEnv("MQVI_ANTIVIRUS_MAX_SCAN_SIZE_MB", "25"), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid MQVI_ANTIVIRUS_MAX_SCAN_SIZE_MB: %w", err)
	}
	avCleanTTL, err := strconv.Atoi(getEnv("MQVI_ANTIVIRUS_CLEAN_CACHE_TTL_HOURS", "24"))
	if err != nil {
		return nil, fmt.Errorf("invalid MQVI_ANTIVIRUS_CLEAN_CACHE_TTL_HOURS: %w", err)
	}
	avInfectedTTL, err := strconv.Atoi(getEnv("MQVI_ANTIVIRUS_INFECTED_CACHE_TTL_DAYS", "30"))
	if err != nil {
		return nil, fmt.Errorf("invalid MQVI_ANTIVIRUS_INFECTED_CACHE_TTL_DAYS: %w", err)
	}
	avCircuitFailures, err := strconv.Atoi(getEnv("MQVI_ANTIVIRUS_CIRCUIT_FAILURE_THRESHOLD", "3"))
	if err != nil {
		return nil, fmt.Errorf("invalid MQVI_ANTIVIRUS_CIRCUIT_FAILURE_THRESHOLD: %w", err)
	}
	avCircuitWindow, err := strconv.Atoi(getEnv("MQVI_ANTIVIRUS_CIRCUIT_WINDOW_SECONDS", "30"))
	if err != nil {
		return nil, fmt.Errorf("invalid MQVI_ANTIVIRUS_CIRCUIT_WINDOW_SECONDS: %w", err)
	}
	avCircuitOpen, err := strconv.Atoi(getEnv("MQVI_ANTIVIRUS_CIRCUIT_OPEN_SECONDS", "10"))
	if err != nil {
		return nil, fmt.Errorf("invalid MQVI_ANTIVIRUS_CIRCUIT_OPEN_SECONDS: %w", err)
	}
	avUnavailablePolicy := strings.ToLower(strings.TrimSpace(getEnv("MQVI_ANTIVIRUS_UNAVAILABLE_POLICY", "allow_with_log")))
	if avUnavailablePolicy != "allow_with_log" && avUnavailablePolicy != "reject" {
		return nil, fmt.Errorf("invalid MQVI_ANTIVIRUS_UNAVAILABLE_POLICY: %s", avUnavailablePolicy)
	}
	avTooLargePolicy := strings.ToLower(strings.TrimSpace(getEnv("MQVI_ANTIVIRUS_TOO_LARGE_POLICY", "skip_with_log")))
	if avTooLargePolicy != "skip_with_log" && avTooLargePolicy != "reject" {
		return nil, fmt.Errorf("invalid MQVI_ANTIVIRUS_TOO_LARGE_POLICY: %s", avTooLargePolicy)
	}
	if avTimeout <= 0 {
		return nil, fmt.Errorf("MQVI_ANTIVIRUS_TIMEOUT_SECONDS must be positive")
	}
	if avMaxScanMB < 0 || avCleanTTL < 0 || avInfectedTTL < 0 || avCircuitFailures < 0 || avCircuitWindow < 0 || avCircuitOpen < 0 {
		return nil, fmt.Errorf("antivirus TTL and circuit values must not be negative")
	}

	jwtSecret := getEnv("JWT_SECRET", "")
	if jwtSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET environment variable is required")
	}

	encKey := getEnv("ENCRYPTION_KEY", "")
	if encKey == "" {
		return nil, fmt.Errorf("ENCRYPTION_KEY environment variable is required (64 hex chars = 32 byte AES-256 key)")
	}

	cfg := &Config{
		Server: ServerConfig{
			Host: getEnv("SERVER_HOST", "0.0.0.0"),
			Port: port,
		},
		Database: DatabaseConfig{
			Path: getEnv("DATABASE_PATH", "./data/mqvi.db"),
		},
		JWT: JWTConfig{
			Secret:             jwtSecret,
			AccessTokenExpiry:  accessExpiry,
			RefreshTokenExpiry: refreshExpiry,
		},
		LiveKit: LiveKitConfig{
			URL:       getEnv("LIVEKIT_URL", "ws://localhost:7880"),
			APIKey:    getEnv("LIVEKIT_API_KEY", ""),
			APISecret: getEnv("LIVEKIT_API_SECRET", ""),
		},
		Upload: UploadConfig{
			Dir:                 getEnv("UPLOAD_DIR", "./data/uploads"),
			MaxSize:             maxSize,
			DefaultQuotaBytes:   defaultQuota,
			PublicURL:           getEnv("MQVI_PUBLIC_FILE_URL", ""),
			SignedURLSecret:     getEnv("MQVI_SIGNED_URL_SECRET", ""),
			SignedURLSecretPrev: getEnv("MQVI_SIGNED_URL_SECRET_PREV", ""),
		},
		Antivirus: AntivirusConfig{
			Enabled:                 avEnabled,
			ClamAVAddr:              getEnv("MQVI_CLAMAV_ADDR", "unix:/run/clamav/clamd.ctl"),
			TimeoutSeconds:          avTimeout,
			MaxScanSizeBytes:        avMaxScanMB * 1024 * 1024,
			UnavailablePolicy:       avUnavailablePolicy,
			TooLargePolicy:          avTooLargePolicy,
			CleanCacheTTLHours:      avCleanTTL,
			InfectedCacheTTLDays:    avInfectedTTL,
			CircuitFailureThreshold: avCircuitFailures,
			CircuitWindowSeconds:    avCircuitWindow,
			CircuitOpenSeconds:      avCircuitOpen,
		},
		FileRateLimit: FileRateLimitConfig{
			UserPerMin: fileRateUser,
			IPPerMin:   fileRateIP,
		},
		Email: EmailConfig{
			ResendAPIKey: getEnv("RESEND_API_KEY", ""),
			FromEmail:    getEnv("RESEND_FROM", ""),
			AppURL:       getEnv("APP_URL", ""),
		},
		Klipy: KlipyConfig{
			APIKey: getEnv("KLIPY_API_KEY", ""),
		},
		EncryptionKey:   encKey,
		HetznerAPIToken: getEnv("HETZNER_API_TOKEN", ""),
	}

	return cfg, nil
}

// Addr returns the listen address (e.g. "0.0.0.0:8080").
func (c *ServerConfig) Addr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return fallback
}
