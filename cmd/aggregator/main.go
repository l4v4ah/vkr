package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"

	"github.com/slava-kov/monitoring-system/internal/config"
	"github.com/slava-kov/monitoring-system/internal/logger"
	"github.com/slava-kov/monitoring-system/internal/metrics"
	natsclient "github.com/slava-kov/monitoring-system/internal/nats"
	"github.com/slava-kov/monitoring-system/internal/otel"
	"github.com/slava-kov/monitoring-system/internal/storage"
)

func main() {
	cfg := config.LoadAggregator()
	log := logger.New("aggregator")
	m := metrics.New("aggregator")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	_, shutdown, err := otel.Setup(ctx, "aggregator", "1.0.0")
	if err != nil {
		log.Fatal("otel setup", zap.Error(err))
	}
	defer func() { _ = shutdown(context.Background()) }()

	db, err := storage.Connect(ctx, cfg.PostgresURL, "migrations")
	if err != nil {
		log.Fatal("postgres connect", zap.Error(err))
	}
	defer db.Close()

	nc, err := natsclient.Connect(cfg.NATSUrl)
	if err != nil {
		log.Fatal("nats connect", zap.Error(err))
	}
	defer nc.Close()

	c := newConsumer(db, nc, m, log)
	log.Info("aggregator started, consuming telemetry")
	c.Run(ctx)
	log.Info("aggregator stopped")
}
