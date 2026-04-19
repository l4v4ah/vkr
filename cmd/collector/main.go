package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/slava-kov/monitoring-system/internal/config"
	"github.com/slava-kov/monitoring-system/internal/logger"
	"github.com/slava-kov/monitoring-system/internal/metrics"
	natsclient "github.com/slava-kov/monitoring-system/internal/nats"
	"github.com/slava-kov/monitoring-system/internal/otel"
)

func main() {
	cfg := config.LoadCollector()
	log := logger.New("collector")
	m := metrics.New("collector")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	tracer, shutdown, err := otel.Setup(ctx, "collector", "1.0.0")
	if err != nil {
		log.Fatal("otel setup", zap.Error(err))
	}
	defer func() { _ = shutdown(context.Background()) }()

	nc, err := natsclient.Connect(cfg.NATSUrl)
	if err != nil {
		log.Fatal("nats connect", zap.Error(err))
	}
	defer nc.Close()

	srv := newServer(cfg.HTTPAddr, nc, m, tracer, log)

	go func() {
		log.Info("collector listening", zap.String("addr", cfg.HTTPAddr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("server error", zap.Error(err))
		}
	}()

	<-ctx.Done()
	log.Info("shutting down collector")

	shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutCtx)
}
