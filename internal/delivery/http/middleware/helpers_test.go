package middleware

import (
	"iter"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

func stringsSplitSeq(s string) iter.Seq[string] { return strings.SplitSeq(s, "\n") }

func newRecorder() *httptest.ResponseRecorder { return httptest.NewRecorder() }

func newRequestWithSpan(t *testing.T, sc trace.SpanContext) *http.Request {
	t.Helper()

	ctx := trace.ContextWithSpanContext(t.Context(), sc)

	return httptest.NewRequestWithContext(ctx, http.MethodGet, "/x", nil)
}
