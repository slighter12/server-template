package otel

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

func newTestLogger(buf *bytes.Buffer, level slog.Level) *slog.Logger {
	return slog.New(NewTraceHandler(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: level})))
}

func decode(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()

	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("decode %q: %v", buf.String(), err)
	}

	return m
}

func validCtx() context.Context {
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		SpanID:     trace.SpanID{1, 2, 3, 4, 5, 6, 7, 8},
		TraceFlags: trace.FlagsSampled,
	})

	return trace.ContextWithSpanContext(context.Background(), sc)
}

func TestTraceHandler_AddsIDs(t *testing.T) {
	var buf bytes.Buffer
	newTestLogger(&buf, slog.LevelInfo).InfoContext(validCtx(), "hi")

	m := decode(t, &buf)
	if m["trace_id"] != "0102030405060708090a0b0c0d0e0f10" || m["span_id"] != "0102030405060708" {
		t.Errorf("unexpected ids: %v", m)
	}
}

func TestTraceHandler_NoSpan(t *testing.T) {
	var buf bytes.Buffer
	newTestLogger(&buf, slog.LevelInfo).InfoContext(context.Background(), "hi")

	m := decode(t, &buf)
	if _, ok := m["trace_id"]; ok {
		t.Errorf("trace_id must be absent: %v", m)
	}
}

func TestTraceHandler_Enabled(t *testing.T) {
	var buf bytes.Buffer
	l := newTestLogger(&buf, slog.LevelWarn)
	if l.Enabled(context.Background(), slog.LevelInfo) {
		t.Error("info must be disabled")
	}
	l.InfoContext(validCtx(), "dropped")
	if buf.Len() != 0 {
		t.Errorf("unexpected output %q", buf.String())
	}
}

func TestTraceHandler_WithAttrsAndGroup(t *testing.T) {
	var buf bytes.Buffer
	l := newTestLogger(&buf, slog.LevelInfo).With("svc", "x").WithGroup("g")
	l.InfoContext(validCtx(), "hi", "k", "v")

	m := decode(t, &buf)
	if m["svc"] != "x" {
		t.Errorf("svc missing: %v", m)
	}
	g, ok := m["g"].(map[string]any)
	if !ok || g["k"] != "v" {
		t.Fatalf("group missing: %v", m)
	}
	// Record attrs are added inside the open group, as with any slog handler.
	if g["trace_id"] == nil || g["span_id"] == nil {
		t.Errorf("ids missing in group: %v", m)
	}
}
