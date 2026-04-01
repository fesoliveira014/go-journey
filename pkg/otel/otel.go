package otel

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sync"
	"time"

	otelgo "go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// Init sets up OpenTelemetry tracing and metrics with OTLP/gRPC exporters.
// It registers a TracerProvider and MeterProvider as global OTel providers and
// configures the default slog logger to inject trace/span IDs into log records.
//
// Returns a shutdown function that must be called (e.g. via defer) to flush and
// close the exporters cleanly before the process exits.
func Init(ctx context.Context, serviceName, serviceVersion, collectorEndpoint string) (func(context.Context) error, error) {
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
			semconv.ServiceVersionKey.String(serviceVersion),
		),
	)
	if err != nil {
		return nil, err
	}

	traceExporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(collectorEndpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
	)
	otelgo.SetTracerProvider(tp)
	otelgo.SetTextMapPropagator(propagation.TraceContext{})

	metricExporter, err := otlpmetricgrpc.New(ctx,
		otlpmetricgrpc.WithEndpoint(collectorEndpoint),
		otlpmetricgrpc.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}

	mp := metric.NewMeterProvider(
		metric.WithReader(metric.NewPeriodicReader(metricExporter, metric.WithInterval(30*time.Second))),
		metric.WithResource(res),
	)
	otelgo.SetMeterProvider(mp)

	slog.SetDefault(slog.New(NewTraceLogHandler(os.Stdout)))

	var once sync.Once
	var shutdownErr error
	shutdown := func(ctx context.Context) error {
		once.Do(func() {
			shutdownErr = errors.Join(tp.Shutdown(ctx), mp.Shutdown(ctx))
			if shutdownErr != nil {
				slog.Default().Error("otel shutdown error", "err", shutdownErr)
			}
		})
		return shutdownErr
	}

	return shutdown, nil
}
