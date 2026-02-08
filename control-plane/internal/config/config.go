package config

import "os"

type Config struct {
	Port               string
	GatewayInternalURL string
}

func Load() *Config {
	return &Config{
		Port:               getEnv("PORT", "8080"),
		GatewayInternalURL: getEnv("GATEWAY_INTERNAL_URL", "http://localhost:9091"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
