package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port          string
	GinMode       string
	LogLevel      string
	DatabaseURL   string
	AdminToken    string
	InternalToken string
	AIServiceURL  string
	CORSOrigins   []string

	// Rate limit umum per sesi (atau per IP jika tanpa sesi).
	RateLimitRPS   float64
	RateLimitBurst int
	// Rate limit pembuatan sesi per IP, per menit.
	SessionCreatePerMinute int
}

func Load() (*Config, error) {
	cfg := &Config{
		Port:          getEnv("BACKEND_PORT", "8080"),
		GinMode:       getEnv("GIN_MODE", "debug"),
		LogLevel:      getEnv("LOG_LEVEL", "info"),
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		AdminToken:    os.Getenv("ADMIN_TOKEN"),
		InternalToken: os.Getenv("INTERNAL_TOKEN"),
		AIServiceURL:  getEnv("AI_SERVICE_URL", "http://ai-service:8000"),
		CORSOrigins:   splitCSV(getEnv("CORS_ORIGINS", "http://localhost:3000")),
	}
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL wajib diisi")
	}

	var err error
	if cfg.RateLimitRPS, err = getFloat("RATE_LIMIT_RPS", 2); err != nil {
		return nil, err
	}
	if cfg.RateLimitBurst, err = getInt("RATE_LIMIT_BURST", 30); err != nil {
		return nil, err
	}
	if cfg.SessionCreatePerMinute, err = getInt("SESSION_CREATE_PER_MINUTE", 10); err != nil {
		return nil, err
	}
	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getInt(key string, fallback int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("%s harus bilangan bulat positif", key)
	}
	return n, nil
}

func getFloat(key string, fallback float64) (float64, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil || f <= 0 {
		return 0, fmt.Errorf("%s harus angka positif", key)
	}
	return f, nil
}

func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
