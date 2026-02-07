package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/RenatoCabral2022/WHATS-SERVICE/webrtc-gateway/internal/config"
)

func main() {
	cfg := config.Load()
	log.Printf("webrtc-gateway starting, listen=%s, asr=%s, tts=%s",
		cfg.ListenAddr, cfg.ASRAddr, cfg.TTSAddr)

	// TODO: initialize gateway, gRPC clients, start accepting WebRTC connections

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("shutting down")
}
