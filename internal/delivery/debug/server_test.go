package debug

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"server-template/config"

	"go.uber.org/fx"
)

func get(t *testing.T, url string) (*http.Response, string) {
	t.Helper()
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)

	return resp, string(b)
}

func TestEndpoints(t *testing.T) {
	s := newServer("127.0.0.1:0", nil)
	ts := httptest.NewServer(s.handler())
	defer ts.Close()

	// Before the recorder starts.
	if resp, _ := get(t, ts.URL+"/debug/flightrecorder"); resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", resp.StatusCode)
	}

	if err := s.fr.Start(); err != nil {
		t.Fatal(err)
	}
	defer s.fr.Stop()

	resp, body := get(t, ts.URL+"/debug/pprof/")
	if resp.StatusCode != 200 || !strings.Contains(body, "goroutineleak") {
		t.Fatalf("pprof index: %d, has goroutineleak=%v", resp.StatusCode, strings.Contains(body, "goroutineleak"))
	}

	resp, body = get(t, ts.URL+"/debug/flightrecorder")
	if resp.StatusCode != 200 || len(body) == 0 {
		t.Fatalf("flight: %d, %d bytes", resp.StatusCode, len(body))
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/octet-stream" {
		t.Fatalf("content-type = %q", ct)
	}
	if !strings.Contains(resp.Header.Get("Content-Disposition"), "filename=") {
		t.Fatal("missing filename")
	}
}

func TestServeAndShutdown(t *testing.T) {
	s := newServer("127.0.0.1:0", nil)
	errc := make(chan error, 1)
	go func() { errc <- s.Serve(t.Context()) }()

	addr := <-s.ready
	if resp, _ := get(t, "http://"+addr.String()+"/debug/pprof/"); resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if resp, body := get(t, "http://"+addr.String()+"/debug/flightrecorder"); resp.StatusCode != 200 || body == "" {
		t.Fatalf("flight: %d", resp.StatusCode)
	}

	if err := s.shutdown(t.Context()); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-errc:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not return")
	}
}

func TestDisabledIsNoop(t *testing.T) {
	var cfg config.Config
	d := NewDebugServer(Params{Lifecycle: fx.Lifecycle(nil), Config: &cfg})
	if _, ok := d.(noop); !ok {
		t.Fatalf("got %T", d)
	}
	if err := d.Serve(t.Context()); err != nil {
		t.Fatal(err)
	}
}
