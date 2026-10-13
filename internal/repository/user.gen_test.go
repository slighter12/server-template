package repository

import (
	"context"
	"errors"
	"strings"
	"testing"

	"server-template/internal/domain/entity"
	"server-template/internal/domain/repository"
	"server-template/internal/pkg/apperr"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

type stubUserRepo struct {
	repository.UserRepository
	err error
}

func (s stubUserRepo) FindByEmail(context.Context, string) (*entity.User, error) { return nil, s.err }

func setupTracing(t *testing.T) *tracetest.InMemoryExporter {
	t.Helper()

	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	otel.SetTracerProvider(tp)
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	return exp
}

func attrValue(attrs []attribute.KeyValue, key string) string {
	for _, a := range attrs {
		if string(a.Key) == key {
			return a.Value.AsString()
		}
	}

	return ""
}

func TestUserRepositoryProxy_ClientErrorKeepsSpanOK(t *testing.T) {
	exp := setupTracing(t)
	p := ProvideUserRepositoryProxy(true, stubUserRepo{err: apperr.Wrap(errors.New("secret@example.com row"), apperr.ErrNotFound)})

	_, err := p.FindByEmail(context.Background(), "secret@example.com")
	if err == nil {
		t.Fatal("expected error")
	}

	spans := exp.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("spans = %d, want 1", len(spans))
	}
	s := spans[0]
	if s.Name != "UserRepository.FindByEmail" {
		t.Errorf("name = %q", s.Name)
	}
	if s.Status.Code != codes.Unset {
		t.Errorf("status = %v, want Unset", s.Status.Code)
	}
	if got := attrValue(s.Attributes, "app.error_code"); got != "NOT_FOUND" {
		t.Errorf("app.error_code = %q", got)
	}
	if got := attrValue(s.Attributes, "app.error_kind"); got != "not_found" {
		t.Errorf("app.error_kind = %q", got)
	}
	if got := attrValue(s.Attributes, "code.function"); got != "FindByEmail" {
		t.Errorf("code.function = %q", got)
	}
	if got := attrValue(s.Attributes, "code.namespace"); got != "UserRepository" {
		t.Errorf("code.namespace = %q", got)
	}
}

func TestUserRepositoryProxy_ServerErrorHasNoRawMessage(t *testing.T) {
	exp := setupTracing(t)
	p := ProvideUserRepositoryProxy(true, stubUserRepo{err: errors.New("dial tcp: user secret@example.com failed")})

	if _, err := p.FindByEmail(context.Background(), "x"); err == nil {
		t.Fatal("expected error")
	}

	s := exp.GetSpans()[0]
	if s.Status.Code != codes.Error || s.Status.Description != "INTERNAL_ERROR" {
		t.Errorf("status = %v %q", s.Status.Code, s.Status.Description)
	}
	if len(s.Events) != 1 || s.Events[0].Name != "exception" {
		t.Fatalf("events = %+v", s.Events)
	}
	if got := attrValue(s.Events[0].Attributes, "exception.type"); got != "*errors.errorString" {
		t.Errorf("exception.type = %q", got)
	}

	var all strings.Builder
	all.WriteString(s.Status.Description)
	for _, a := range s.Attributes {
		all.WriteString(a.Value.String())
	}
	for _, e := range s.Events {
		all.WriteString(e.Name)
		for _, a := range e.Attributes {
			all.WriteString(a.Value.String())
		}
	}
	if strings.Contains(all.String(), "secret@example.com") || strings.Contains(all.String(), "dial tcp") {
		t.Errorf("raw error message leaked into span: %s", all.String())
	}
}

func TestProvideUserRepositoryProxy_DisabledReturnsBase(t *testing.T) {
	base := stubUserRepo{}
	if got := ProvideUserRepositoryProxy(false, base); got != repository.UserRepository(base) {
		t.Errorf("got %T, want base unchanged", got)
	}
}
