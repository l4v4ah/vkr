package config

import (
	"os"
)

// Collector holds configuration for the collector service.
type Collector struct {
	HTTPAddr string // e.g. ":8080"
	NATSUrl  string // e.g. "nats://nats:4222"
}

// Aggregator holds configuration for the aggregator service.
type Aggregator struct {
	NATSUrl     string
	PostgresURL string
}

// API holds configuration for the API gateway service.
type API struct {
	HTTPAddr    string
	PostgresURL string
}

func LoadCollector() Collector {
	return Collector{
		HTTPAddr: env("HTTP_ADDR", ":8080"),
		NATSUrl:  env("NATS_URL", "nats://localhost:4222"),
	}
}

func LoadAggregator() Aggregator {
	return Aggregator{
		NATSUrl:     env("NATS_URL", "nats://localhost:4222"),
		PostgresURL: env("POSTGRES_URL", "postgres://monitoring:monitoring@localhost:5432/monitoring?sslmode=disable"),
	}
}

func LoadAPI() API {
	return API{
		HTTPAddr:    env("HTTP_ADDR", ":8081"),
		PostgresURL: env("POSTGRES_URL", "postgres://monitoring:monitoring@localhost:5432/monitoring?sslmode=disable"),
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
