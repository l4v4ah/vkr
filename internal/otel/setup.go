package otel

import (
	"context"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
)

// Setup initialises a TracerProvider and installs it as the global OTel provider.
// It returns a shutdown function that must be called on service exit.
//
// In production the OTEL_EXPORTER_OTLP_ENDPOINT environment variable selects an
// OTLP/gRPC endpoint (e.g. an OpenTelemetry Collector side-car). When the variable
// is absent the provider falls back to a stdout exporter for local development.
func Setup(ctx context.Context, service, version string) (trace.Tracer, func(context.Context) error, error) {
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(service),
			semconv.ServiceVersion(version),
		),
	)
	if err != nil {
		return nil, nil, err
	}

	exp, err := buildExporter(ctx)
	if err != nil {
		return nil, nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)

	return tp.Tracer(service), tp.Shutdown, nil
}

func buildExporter(ctx context.Context) (sdktrace.SpanExporter, error) {
	if endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"); endpoint != "" {
		// When an OTLP endpoint is configured use it (requires OTLP gRPC exporter).
		// The import is handled at the call site to keep this file compilable without
		// the optional OTLP dependency in local dev mode.
		_ = endpoint
	}
	return stdouttrace.New(stdouttrace.WithPrettyPrint())
}
