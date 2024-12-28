package otel

import (
	"context"
	"fmt"

	"server-template/config"
	"server-template/internal/domain/telemetry"

	"github.com/pkg/errors"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.uber.org/fx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func createOTLPGRPCExporter(ctx context.Context, host string, port int, isSecure bool) (sdktrace.SpanExporter, error) {
	endpoint := fmt.Sprintf("%s:%d", host, port)
	var opts []otlptracegrpc.Option
	opts = append(opts,
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithCompressor("gzip"),
	)

	if !isSecure {
		opts = append(opts, otlptracegrpc.WithInsecure())
		opts = append(opts, otlptracegrpc.WithDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())))
	}

	exporter, err := otlptrace.New(
		ctx,
		otlptracegrpc.NewClient(opts...),
	)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return exporter, nil
}

func createOTLPHTTPExporter(ctx context.Context, host string, port int, isSecure bool) (sdktrace.SpanExporter, error) {
	protocol := map[bool]string{true: "https", false: "http"}[isSecure]
	endpoint := fmt.Sprintf("%s://%s:%d", protocol, host, port)

	var opts []otlptracehttp.Option
	opts = append(opts,
		otlptracehttp.WithEndpoint(endpoint),
		otlptracehttp.WithCompression(otlptracehttp.GzipCompression),
	)

	if !isSecure {
		opts = append(opts, otlptracehttp.WithInsecure())
	}

	exporter, err := otlptrace.New(
		ctx,
		otlptracehttp.NewClient(opts...),
	)

	if err != nil {
		return nil, errors.WithStack(err)
	}

	return exporter, nil
}

func createExporter(ctx context.Context, exporterType telemetry.ExporterType, cfg *config.Config) (sdktrace.SpanExporter, error) {
	if !exporterType.IsValid() {
		return nil, fmt.Errorf("unsupported exporter type: %s", exporterType)
	}

	switch exporterType {
	case telemetry.ExporterOTLPGRPC:
		return createOTLPGRPCExporter(ctx, cfg.Observability.Otel.Host, cfg.Observability.Otel.Port, cfg.Observability.Otel.IsSecure)
	case telemetry.ExporterOTLPHTTP:
		return createOTLPHTTPExporter(ctx, cfg.Observability.Otel.Host, cfg.Observability.Otel.Port, cfg.Observability.Otel.IsSecure)
	default:
		return nil, fmt.Errorf("unhandled exporter type: %s", exporterType)
	}
}

func NewTracer(
	ctx context.Context,
	lc fx.Lifecycle,
	cnf *config.Config,
) error {
	if !cnf.Observability.Otel.Enable {
		return nil
	}

	res, err := resource.New(
		ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(cnf.Env.ServiceName),
			semconv.ServiceVersionKey.String("v1.0.0"),
			semconv.DeploymentEnvironmentKey.String(cnf.Env.Env),
		),
	)
	if err != nil {
		return errors.WithStack(err)
	}

	exporterType := telemetry.ExporterType(cnf.Observability.Otel.Exporter)
	exporter, err := createExporter(ctx, exporterType, cnf)
	if err != nil {
		return errors.WithStack(err)
	}

	tracer := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)

	otel.SetTracerProvider(tracer)

	lc.Append(fx.Hook{
		OnStop: func(stopCtx context.Context) error {
			return tracer.Shutdown(stopCtx)
		},
	})

	return nil
}
