package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"server-template/internal/pkg/observability"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestID(t *testing.T) {
	tests := []struct {
		name     string
		inbound  string
		wantKept bool
	}{
		{name: "generated", inbound: "", wantKept: false},
		{name: "client id kept", inbound: "abc-123_x.y:z", wantKept: true},
		{name: "unsafe id replaced", inbound: "bad id\r\nX: y", wantKept: false},
		{name: "too long replaced", inbound: strings.Repeat("a", maxRequestIDLen+1), wantKept: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, _ := newTestEcho(t, RequestLoggerOptions{})
			var seenID string
			var info *observability.RequestInfo
			var hasLogger bool
			e.GET("/", func(c *echo.Context) error {
				ctx := c.Request().Context()
				seenID = observability.CorrelationIDFromContext(ctx)
				info = observability.RequestInfoFrom(ctx)
				hasLogger = observability.LoggerFromContext(ctx) != nil

				return c.NoContent(http.StatusNoContent)
			})
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
			if tt.inbound != "" {
				req.Header["X-Request-Id"] = []string{tt.inbound}
			}
			rec := httptest.NewRecorder()

			e.ServeHTTP(rec, req)

			require.NotEmpty(t, seenID)
			assert.Equal(t, seenID, rec.Header().Get(observability.HeaderXRequestID))
			assert.Equal(t, tt.wantKept, seenID == tt.inbound)
			assert.NotNil(t, info)
			assert.True(t, hasLogger)
		})
	}
}
