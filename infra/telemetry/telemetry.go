package telemetry

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.41.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

type Telemetry struct {
	Handler   slog.Handler
	shutdowns []func(context.Context) error
	conn      *grpc.ClientConn
}

func (t *Telemetry) Shutdown(ctx context.Context) error {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var errs []error
	for i := len(t.shutdowns) - 1; i >= 0; i-- {
		if err := t.shutdowns[i](shutdownCtx); err != nil {
			errs = append(errs, err)
		}
	}

	if t.conn != nil {
		if err := t.conn.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func Setup(ctx context.Context, cfg Config) (*Telemetry, error) {
	t := &Telemetry{}

	if !cfg.Logs && !cfg.Metrics && !cfg.Traces {
		t.Handler = slog.NewTextHandler(io.Discard, nil)
		return t, nil
	}

	if cfg.TraceSampleRate < 0 || cfg.TraceSampleRate > 1.0 {
		return nil, fmt.Errorf("telemetry: traceSampleRate must be 0.0-1.0, got %f", cfg.TraceSampleRate)
	}
	if cfg.TraceSampleRate == 0 {
		cfg.TraceSampleRate = 1.0
	}

	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
			attribute.String("deployment.environment", cfg.Environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("telemetry: failed to create resource: %w", err)
	}

	var transportCreds credentials.TransportCredentials
	if cfg.Insecure {
		transportCreds = insecure.NewCredentials()
	} else {
		transportCreds = credentials.NewTLS(&tls.Config{})
	}

	conn, err := grpc.NewClient(
		cfg.OTELEndpoint,
		grpc.WithTransportCredentials(transportCreds),
	)
	if err != nil {
		return nil, fmt.Errorf("telemetry: failed to create grpc client: %w", err)
	}
	t.conn = conn

	if cfg.Traces {
		shutdown, err := setupTraces(ctx, res, conn, cfg.TraceSampleRate)
		if err != nil {
			return nil, fmt.Errorf("telemetry: traces: %w", err)
		}
		t.shutdowns = append(t.shutdowns, shutdown)
	}

	if cfg.Metrics {
		shutdown, err := setupMetrics(ctx, res, conn)
		if err != nil {
			return nil, fmt.Errorf("telemetry: metrics: %w", err)
		}
		t.shutdowns = append(t.shutdowns, shutdown)
	}

	if cfg.Logs {
		handler, shutdown, err := setupLogs(ctx, res, conn, cfg)
		if err != nil {
			return nil, fmt.Errorf("telemetry: logs: %w", err)
		}
		t.Handler = handler
		t.shutdowns = append(t.shutdowns, shutdown)
	}

	if t.Handler == nil {
		t.Handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})
	}

	return t, nil
}

func setupTraces(
	ctx context.Context,
	res *resource.Resource,
	conn *grpc.ClientConn,
	sampleRate float64,
) (func(context.Context) error, error) {
	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithGRPCConn(conn),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create exporter: %w", err)
	}

	var sampler sdktrace.Sampler
	if sampleRate >= 1.0 {
		sampler = sdktrace.AlwaysSample()
	} else {
		sampler = sdktrace.ParentBased(
			sdktrace.TraceIDRatioBased(sampleRate),
		)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)

	otel.SetTracerProvider(tp)

	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp.Shutdown, nil
}

func setupMetrics(
	ctx context.Context,
	res *resource.Resource,
	conn *grpc.ClientConn,
) (func(context.Context) error, error) {
	exporter, err := otlpmetricgrpc.New(ctx,
		otlpmetricgrpc.WithGRPCConn(conn),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create exporter: %w", err)
	}

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(
			sdkmetric.NewPeriodicReader(exporter),
		),
		sdkmetric.WithResource(res),
	)

	otel.SetMeterProvider(mp)

	return mp.Shutdown, nil
}

func setupLogs(
	ctx context.Context,
	res *resource.Resource,
	conn *grpc.ClientConn,
	cfg Config,
) (slog.Handler, func(context.Context) error, error) {
	exporter, err := otlploggrpc.New(ctx,
		otlploggrpc.WithGRPCConn(conn),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create exporter: %w", err)
	}

	lp := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(exporter)),
		sdklog.WithResource(res),
	)

	var otelHandler slog.Handler
	otelHandler = otelslog.NewHandler(
		cfg.ServiceName,
		otelslog.WithLoggerProvider(lp),
	)

	if cfg.SampleLogs {
		minLevel := cfg.UnsampledLogLevel
		if minLevel == 0 {
			minLevel = slog.LevelError
		}
		otelHandler = newSampledLogHandler(otelHandler, minLevel)
	}

	var handler slog.Handler

	if cfg.LogToStdout {
		stdoutHandler := slog.NewJSONHandler(
			os.Stdout,
			&slog.HandlerOptions{
				Level:     cfg.StdoutLogLevel,
				AddSource: true,
			},
		)
		handler = &fanOutHandler{
			handlers: []slog.Handler{stdoutHandler, otelHandler},
		}
	} else {

		handler = otelHandler
	}

	return handler, lp.Shutdown, nil
}

func HandleError(span trace.Span, desc string, err error) {
	span.RecordError(err)
	span.SetStatus(codes.Error, desc)
}

func MergeAttrs(base []attribute.KeyValue, extra ...attribute.KeyValue) []attribute.KeyValue {
	return append(base, extra...)
}
