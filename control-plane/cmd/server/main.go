package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/RenatoCabral2022/WhatsWebService/control-plane/internal/config"
	"github.com/RenatoCabral2022/WhatsWebService/control-plane/internal/handler"
	"github.com/RenatoCabral2022/WhatsWebService/control-plane/internal/middleware"
)

func main() {
	cfg := config.Load()
	h := handler.NewHandlers(cfg.GatewayInternalURL)

	r := chi.NewRouter()
	r.Use(chimw.RealIP)
	r.Use(middleware.RequestID)
	r.Use(middleware.Logging)
	r.Use(chimw.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	r.Get("/healthz", h.Health)

	r.Route("/v1", func(r chi.Router) {
		r.Route("/sessions", func(r chi.Router) {
			r.Post("/", h.CreateSession)
			r.Route("/{sessionId}", func(r chi.Router) {
				r.Delete("/", h.DeleteSession)
				r.Post("/webrtc/answer", h.PostWebRTCAnswer)
			})
		})
	})

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 20 * time.Second,
	}

	go func() {
		log.Printf("control-plane listening on :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
}
