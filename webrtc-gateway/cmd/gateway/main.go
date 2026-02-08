package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"github.com/RenatoCabral2022/WhatsWebService/webrtc-gateway/internal/config"
	"github.com/RenatoCabral2022/WhatsWebService/webrtc-gateway/internal/gateway"
	_ "github.com/RenatoCabral2022/WhatsWebService/webrtc-gateway/internal/metrics" // register metrics
)

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	cfg := config.Load()
	logger.Info("webrtc-gateway starting",
		zap.String("listen", cfg.ListenAddr),
		zap.String("internalAPI", cfg.InternalAPIAddr),
		zap.String("metrics", cfg.MetricsAddr),
		zap.String("asr", cfg.ASRAddr),
		zap.String("tts", cfg.TTSAddr),
		zap.Int("maxSessions", cfg.MaxSessions),
		zap.Int("maxInferenceConcurrency", cfg.MaxInferenceConcurrency),
	)

	gw, err := gateway.New(cfg, logger)
	if err != nil {
		logger.Fatal("failed to create gateway", zap.Error(err))
	}

	// Internal API server
	srv := &http.Server{
		Addr:         cfg.InternalAPIAddr,
		Handler:      gw.InternalHandler(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 20 * time.Second,
	}

	go func() {
		logger.Info("internal API listening", zap.String("addr", cfg.InternalAPIAddr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("internal API failed", zap.Error(err))
		}
	}()

	// Metrics server (Prometheus)
	metricsSrv := &http.Server{
		Addr:    cfg.MetricsAddr,
		Handler: promhttp.Handler(),
	}

	go func() {
		logger.Info("metrics server listening", zap.String("addr", cfg.MetricsAddr))
		if err := metricsSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("metrics server failed", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down")
	gw.Shutdown()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
	metricsSrv.Shutdown(ctx)
}
