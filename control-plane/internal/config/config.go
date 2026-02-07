package config

import "os"

type Config struct {
	Port        string
	GatewayAddr string
}

func Load() *Config {
	return &Config{
		Port:        getEnv("PORT", "8080"),
		GatewayAddr: getEnv("GATEWAY_ADDR", "localhost:9090"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
