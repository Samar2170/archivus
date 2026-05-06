package logging

import (
	"archivus/internal/config"
	"context"
	"fmt"
	"path/filepath"

	"github.com/natefinch/lumberjack"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

// InitProvider initializes the OpenTelemetry tracer provider.
// It uses a stdout exporter for demonstration/local logging purposes.
// Returns a shutdown function that should be called when the service terminates.
func InitProvider(serviceName, serviceVersion string) (func(context.Context) error, error) {
	// Create resource describing this application.
	res, err := resource.New(context.Background(),
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(serviceVersion),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	traceLogFilePath := filepath.Join(config.Config.LogsDir, "traces.log")
	traceLogFile := &lumberjack.Logger{
		Filename:   traceLogFilePath,
		MaxSize:    10,
		MaxBackups: 3,
		MaxAge:     28,
		Compress:   false,
	}

	// Create stdout exporter to print traces to stdout.
	// You might replace this with an OTLP exporter in production.
	traceExporter, err := stdouttrace.New(
		stdouttrace.WithPrettyPrint(),
		stdouttrace.WithWriter(traceLogFile),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create trace exporter: %w", err)
	}

	// Create TracerProvider with the exporter and resource.
	bsp := sdktrace.NewBatchSpanProcessor(traceExporter)
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithSpanProcessor(bsp),
		sdktrace.WithResource(res),
	)

	// Set the global TracerProvider and Propagator.
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Return a function to shutdown the TracerProvider.
	return tp.Shutdown, nil
}
