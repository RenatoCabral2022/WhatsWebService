package config

import "os"

type Config struct {
	ListenAddr    string
	ASRAddr       string
	TTSAddr       string
	RingBufferSec int
}

func Load() *Config {
	return &Config{
		ListenAddr:    getEnv("LISTEN_ADDR", ":9090"),
		ASRAddr:       getEnv("ASR_ADDR", "localhost:50051"),
		TTSAddr:       getEnv("TTS_ADDR", "localhost:50052"),
		RingBufferSec: 30,
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
