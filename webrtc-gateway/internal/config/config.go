package config

import (
	"os"
	"strings"
)

type Config struct {
	ListenAddr      string
	InternalAPIAddr string
	ASRAddr         string
	TTSAddr         string
	RingBufferSec   int
	STUNServers     []string
}

func Load() *Config {
	return &Config{
		ListenAddr:      getEnv("LISTEN_ADDR", ":9090"),
		InternalAPIAddr: getEnv("INTERNAL_API_ADDR", ":9091"),
		ASRAddr:         getEnv("ASR_ADDR", "localhost:50051"),
		TTSAddr:         getEnv("TTS_ADDR", "localhost:50052"),
		RingBufferSec:   30,
		STUNServers:     getEnvList("STUN_SERVERS", []string{"stun:stun.l.google.com:19302"}),
	}
}

func getEnvList(key string, fallback []string) []string {
	if v := os.Getenv(key); v != "" {
		return strings.Split(v, ",")
	}
	return fallback
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
