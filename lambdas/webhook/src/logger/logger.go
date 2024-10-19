package logger

import (
	"context"
	"os"
	"runtime"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"go.opentelemetry.io/contrib/propagators/aws/xray"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

const (
	TRACE_ENDPOINT_ENV_VAR_KEY = "TRACE_ENDPOINT"
	SERVICE_NAME_ENV_VAR_KEY   = "SERVICE_NAME"
)

var (
	instance *zap.Logger
	once     sync.Once
)

func GetLogger() *zap.Logger {
	once.Do(func() {
		prodEncoderConfig := zap.NewProductionEncoderConfig()
		prodEncoderConfig.TimeKey = "timestamp"
		prodEncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

		config := zap.Config{
			Level:            zap.NewAtomicLevelAt(zap.DebugLevel),
			Sampling:         nil,
			Encoding:         "json",
			EncoderConfig:    prodEncoderConfig,
			OutputPaths:      []string{"stderr"},
			ErrorOutputPaths: []string{"stderr"},
			InitialFields:    map[string]interface{}{"go_version": runtime.Version()},
		}

		instance = zap.Must(config.Build())
	})

	return instance
}

func WithTraceContext(logger zap.SugaredLogger, traceID string, spanID string) *zap.SugaredLogger {
	return logger.With(
		zap.String("traceID", traceID),
		zap.String("spanID", spanID),
	)
}

func InitializeTracer(ctx context.Context) (*sdktrace.TracerProvider, error) {
	traceEndpoint := os.Getenv(TRACE_ENDPOINT_ENV_VAR_KEY)
	serviceNameKey := os.Getenv(SERVICE_NAME_ENV_VAR_KEY)
	dev := os.Getenv("environment") == "dev"

	var client otlptrace.Client
	if dev { //dev should use insecure for testing
		client = otlptracehttp.NewClient(
			otlptracehttp.WithEndpoint(traceEndpoint),
			otlptracehttp.WithInsecure(),
			otlptracehttp.WithURLPath("/TraceSegments"),
			otlptracehttp.WithTimeout(time.Second*30))
	} else {
		client = otlptracehttp.NewClient(
			otlptracehttp.WithEndpoint(traceEndpoint),
			otlptracehttp.WithURLPath("/TraceSegments"),
			otlptracehttp.WithTimeout(time.Second*30))
	}

	exporter, err := otlptrace.New(ctx, client)
	if err != nil {
		return nil, err
	}

	traceProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(serviceNameKey),
		)),
	)
	otel.SetTracerProvider(traceProvider)
	otel.SetTextMapPropagator(xray.Propagator{})
	return traceProvider, nil
}
