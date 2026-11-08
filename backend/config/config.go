package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds all application configuration loaded from environment variables.
// The application fails fast on startup if any required variable is missing.
type Config struct {
	// Server
	Port string

	// Database
	DatabaseURL string

	// JWT (RS256 PEM-encoded keys)
	JWTPrivateKey string
	JWTPublicKey  string

	// Anthropic
	AnthropicAPIKey string

	// S3-compatible bucket (optional — omit to use local disk storage)
	AWSEndpointURL  string // AWS_ENDPOINT_URL
	AWSRegion       string // AWS_DEFAULT_REGION
	AWSAccessKeyID  string // AWS_ACCESS_KEY_ID
	AWSSecretKey    string // AWS_SECRET_ACCESS_KEY
	AWSS3BucketName string // AWS_S3_BUCKET_NAME

	// Local storage (used when AWS vars are absent)
	LocalUploadDir string // LOCAL_UPLOAD_DIR  (default: ./uploads)
	LocalUploadURL string // LOCAL_UPLOAD_URL  (default: http://localhost:<PORT>/uploads)

	// CORS — comma-separated list of allowed origins
	FrontendURL  string
	FrontendURLs []string

	// WebAuthn / Passkeys
	WebAuthnRPID     string // WEBAUTHN_RPID     — domain only, e.g. "myapp.up.railway.app"
	WebAuthnRPOrigin string // WEBAUTHN_RPORIGIN — full URL, e.g. "https://myapp.up.railway.app"
	WebAuthnRPName   string // WEBAUTHN_RPNAME   — display name, default "FinanceAI"

	// Exchange rate API (optional — free tier, no auth required)
	ExchangeRateAPIURL string // EXCHANGE_RATE_API_URL (default: https://api.exchangerate-api.com/v4/latest)

	// Derived / optional
	RateLimitRequests int // default: 5
	RateLimitSeconds  int // default: 900 (15 min)
}

// Load reads all environment variables and returns a Config.
// It collects every missing variable before returning the error,
// so the operator sees all problems at once instead of one at a time.
func Load() (*Config, error) {
	var missing []string

	require := func(key string) string {
		v := os.Getenv(key)
		if v == "" {
			missing = append(missing, key)
		}
		return v
	}

	cfg := &Config{
		Port:            getEnvOrDefault("PORT", "8080"),
		DatabaseURL:     require("DATABASE_URL"),
		JWTPrivateKey:   normalizePEM(require("JWT_PRIVATE_KEY")),
		JWTPublicKey:    normalizePEM(require("JWT_PUBLIC_KEY")),
		AnthropicAPIKey: require("ANTHROPIC_API_KEY"),
		// AWS vars are optional — omit all to fall back to local disk storage
		AWSEndpointURL:  os.Getenv("AWS_ENDPOINT_URL"),
		AWSRegion:       getEnvOrDefault("AWS_DEFAULT_REGION", "auto"),
		AWSAccessKeyID:  os.Getenv("AWS_ACCESS_KEY_ID"),
		AWSSecretKey:    os.Getenv("AWS_SECRET_ACCESS_KEY"),
		AWSS3BucketName: os.Getenv("AWS_S3_BUCKET_NAME"),
		FrontendURL:  require("FRONTEND_URL"),
		WebAuthnRPID:     os.Getenv("WEBAUTHN_RPID"),
		WebAuthnRPOrigin: os.Getenv("WEBAUTHN_RPORIGIN"),
		WebAuthnRPName:   getEnvOrDefault("WEBAUTHN_RPNAME", "FinanceAI"),
	}

	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required environment variables: %v", missing)
	}

	cfg.FrontendURLs = splitTrim(cfg.FrontendURL)
	cfg.RateLimitRequests = getEnvInt("RATE_LIMIT_REQUESTS", 5)
	cfg.RateLimitSeconds = getEnvInt("RATE_LIMIT_SECONDS", 900)
	cfg.ExchangeRateAPIURL = getEnvOrDefault("EXCHANGE_RATE_API_URL", "https://api.exchangerate-api.com/v4/latest")

	port := cfg.Port
	cfg.LocalUploadDir = getEnvOrDefault("LOCAL_UPLOAD_DIR", "./uploads")
	cfg.LocalUploadURL = getEnvOrDefault("LOCAL_UPLOAD_URL", "http://localhost:"+port+"/uploads")

	return cfg, nil
}

// normalizePEM converts literal "\n" sequences (common when setting PEM keys
// as single-line env vars in Railway/Heroku) into real newlines.
func normalizePEM(s string) string {
	return strings.ReplaceAll(s, `\n`, "\n")
}

func getEnvOrDefault(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

func splitTrim(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func getEnvInt(key string, defaultValue int) int {
	v := os.Getenv(key)
	if v == "" {
		return defaultValue
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return defaultValue
	}
	return n
}
