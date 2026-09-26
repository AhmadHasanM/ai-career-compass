package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Port          string
	GinMode       string
	DatabaseURL   string
	AdminToken    string
	InternalToken string
	AIServiceURL  string
	CORSOrigins   []string
}

func Load() (*Config, error) {
	cfg := &Config{
		Port:          getEnv("BACKEND_PORT", "8080"),
		GinMode:       getEnv("GIN_MODE", "debug"),
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		AdminToken:    os.Getenv("ADMIN_TOKEN"),
		InternalToken: os.Getenv("INTERNAL_TOKEN"),
		AIServiceURL:  getEnv("AI_SERVICE_URL", "http://ai-service:8000"),
		CORSOrigins:   splitCSV(getEnv("CORS_ORIGINS", "http://localhost:3000")),
	}
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL wajib diisi")
	}
	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
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
