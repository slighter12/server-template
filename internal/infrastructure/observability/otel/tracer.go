package otel

import (
	"context"
	"fmt"
	"runtime/debug"

	"server-template/config"
	"server-template/internal/domain/telemetry"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.uber.org/fx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Params 定義 tracer 所需的參數
type Params struct {
	fx.In

	Lifecycle fx.Lifecycle
	Config    *config.Config
}

// New 創建並初始化 OpenTelemetry tracer
func New(ctx context.Context, params Params) error {
	if !params.Config.Observability.Otel.Enable {
		return nil
	}

	res, err := resource.New(
		ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(params.Config.Env.ServiceName),
			semconv.ServiceVersionKey.String(serviceVersion()),
			semconv.DeploymentEnvironmentNameKey.String(params.Config.Env.Env),
		),
	)
	if err != nil {
		return fmt.Errorf("otel: %w", err)
	}

	exporterType := telemetry.ExporterType(params.Config.Observability.Otel.Exporter)
	exporter, err := createExporter(ctx, exporterType, params.Config)
	if err != nil {
		return fmt.Errorf("otel: %w", err)
	}

	tracer := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)

	otel.SetTracerProvider(tracer)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	params.Lifecycle.Append(fx.Hook{
		OnStop: func(stopCtx context.Context) error {
			return tracer.Shutdown(stopCtx)
		},
	})

	return nil
}

// serviceVersion returns the main module version from build info, or "dev".
func serviceVersion() string {
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}

	return "dev"
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
		return nil, fmt.Errorf("otel: %w", err)
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
		return nil, fmt.Errorf("otel: %w", err)
	}

	return exporter, nil
}
