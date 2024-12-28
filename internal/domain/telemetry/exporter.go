package telemetry

type ExporterType string

const (
	ExporterOTLPGRPC ExporterType = "otlp-grpc"
	ExporterOTLPHTTP ExporterType = "otlp-http"
)

func (e ExporterType) IsValid() bool {
	switch e {
	case ExporterOTLPGRPC, ExporterOTLPHTTP:
		return true
	default:
		return false
	}
}
