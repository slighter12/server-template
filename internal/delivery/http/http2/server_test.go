package http2

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/synctest"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"server-template/config"
)

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestNewHTTPServerSettings(t *testing.T) {
	cfg := &config.Config{}
	cfg.HTTP.Port = 18080
	cfg.HTTP.MaxHeaderValueCount = 50
	cfg.HTTP.Timeouts.ReadTimeout = 1
	cfg.HTTP.Timeouts.ReadHeaderTimeout = 2
	cfg.HTTP.Timeouts.WriteTimeout = 3
	cfg.HTTP.Timeouts.IdleTimeout = 4

	srv := newHTTPServer(cfg, http.NotFoundHandler())

	assert.Equal(t, ":18080", srv.Addr)
	assert.EqualValues(t, 1, srv.ReadTimeout)
	assert.EqualValues(t, 2, srv.ReadHeaderTimeout)
	assert.EqualValues(t, 3, srv.WriteTimeout)
	assert.EqualValues(t, 4, srv.IdleTimeout)
	assert.Equal(t, 50, srv.MaxHeaderValueCount)
	assert.True(t, srv.Protocols.HTTP1())
	assert.True(t, srv.Protocols.UnencryptedHTTP2())

	assert.Zero(t, newHTTPServer(&config.Config{}, nil).MaxHeaderValueCount)
}

func TestServeHTTP1AndH2C(t *testing.T) {
	e := echo.New()
	e.GET("/probe", func(c *echo.Context) error { return c.String(http.StatusOK, "ok") })

	s := &server{logger: discardLogger(), http: newHTTPServer(&config.Config{}, e)}
	lc := net.ListenConfig{}
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	done := make(chan error, 1)
	go func() { done <- s.serve(ln) }()
	t.Cleanup(func() {
		require.NoError(t, s.stop(context.Background()))
		require.NoError(t, <-done)
	})

	url := "http://" + ln.Addr().String() + "/probe"
	tests := []struct {
		name      string
		configure func(*http.Protocols)
		wantMajor int
	}{
		{"http1", func(p *http.Protocols) { p.SetHTTP1(true) }, 1},
		{"h2c", func(p *http.Protocols) { p.SetUnencryptedHTTP2(true) }, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var protocols http.Protocols
			tt.configure(&protocols)
			client := &http.Client{Transport: &http.Transport{Protocols: &protocols}}
			t.Cleanup(client.CloseIdleConnections)

			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
			require.NoError(t, err)
			resp, err := client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Equal(t, tt.wantMajor, resp.ProtoMajor)
			assert.Equal(t, "ok", string(body))
		})
	}
}

func TestShutdownWaitsForInflightRequest(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		started := make(chan struct{})
		release := make(chan struct{})
		ts := httptest.NewTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			close(started)
			<-release
			_, _ = io.WriteString(w, "done")
		}))
		s := &server{logger: discardLogger(), http: ts.Config}

		type result struct {
			body string
			err  error
		}
		resCh := make(chan result, 1)
		go func() {
			resp, err := ts.Client().Get(ts.URL)
			if err != nil {
				resCh <- result{err: err}

				return
			}
			defer resp.Body.Close()
			b, err := io.ReadAll(resp.Body)
			resCh <- result{body: string(b), err: err}
		}()
		<-started

		stopped := make(chan error, 1)
		go func() { stopped <- s.stop(t.Context()) }()

		synctest.Wait()
		select {
		case <-stopped:
			t.Fatal("shutdown returned while a request was still in flight")
		default:
		}

		close(release)
		require.NoError(t, <-stopped)
		res := <-resCh
		require.NoError(t, res.err)
		assert.Equal(t, "done", res.body)
	})
}
