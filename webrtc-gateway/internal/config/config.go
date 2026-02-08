package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	ListenAddr      string
	InternalAPIAddr string
	MetricsAddr     string
	ASRAddr         string
	TTSAddr         string
	RingBufferSec   int
	STUNServers     []string

	// Backpressure & limits
	MaxSessions             int
	MaxLookbackSec          int
	ActionTimeoutSec        int
	MaxInferenceConcurrency int
}

func Load() *Config {
	return &Config{
		ListenAddr:              getEnv("LISTEN_ADDR", ":9090"),
		InternalAPIAddr:         getEnv("INTERNAL_API_ADDR", ":9091"),
		MetricsAddr:             getEnv("METRICS_ADDR", ":9092"),
		ASRAddr:                 getEnv("ASR_ADDR", "localhost:50051"),
		TTSAddr:                 getEnv("TTS_ADDR", "localhost:50052"),
		RingBufferSec:           getEnvInt("RING_BUFFER_SEC", 60),
		STUNServers:             getEnvList("STUN_SERVERS", []string{"stun:stun.l.google.com:19302"}),
		MaxSessions:             getEnvInt("MAX_SESSIONS", 100),
		MaxLookbackSec:          getEnvInt("MAX_LOOKBACK_SEC", 60),
		ActionTimeoutSec:        getEnvInt("ACTION_TIMEOUT_SEC", 60),
		MaxInferenceConcurrency: getEnvInt("MAX_INFERENCE_CONCURRENCY", 4),
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

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
