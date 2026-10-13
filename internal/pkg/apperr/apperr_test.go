package apperr_test

import (
	"errors"
	"strings"
	"testing"

	"server-template/internal/pkg/apperr"
)

var errUserNotFound = apperr.New(apperr.NotFound, "USER_NOT_FOUND", "user not found")

func TestWrapKeepsCauseAndIdentity(t *testing.T) {
	cause := errors.New("record not found")
	err := apperr.Wrap(cause, errUserNotFound)

	if !errors.Is(err, errUserNotFound) {
		t.Fatal("wrapped error should match its sentinel")
	}
	if !errors.Is(err, cause) {
		t.Fatal("wrapped error should keep the cause chain")
	}
	if errors.Is(err, apperr.ErrNotFound) {
		t.Fatal("different codes must not match")
	}
	if !strings.Contains(err.Stack(), "TestWrapKeepsCauseAndIdentity") {
		t.Fatalf("stack should point at the caller, got:\n%s", err.Stack())
	}
	if errUserNotFound.Stack() != "" || errUserNotFound.Cause() != nil {
		t.Fatal("Wrap must not mutate the sentinel")
	}
}

func TestWrapKeepsInnermostStack(t *testing.T) {
	inner := apperr.Wrap(errors.New("boom"), apperr.ErrInternal)
	outer := apperr.Wrap(inner, apperr.ErrUnavailable)

	if outer.Stack() != inner.Stack() {
		t.Fatal("outer wrap should reuse the inner stack")
	}
	if outer.Kind != apperr.Unavailable {
		t.Fatalf("outer kind = %v", outer.Kind)
	}
}

func TestFrom(t *testing.T) {
	if apperr.From(nil) != nil {
		t.Fatal("From(nil) should be nil")
	}

	plain := errors.New("db down")
	got := apperr.From(plain)
	if got.Kind != apperr.Internal || !errors.Is(got, plain) {
		t.Fatalf("plain error should become internal with cause, got %+v", got)
	}

	app := apperr.Wrap(plain, errUserNotFound)
	if apperr.From(app) != app {
		t.Fatal("From should return an existing *Error unchanged")
	}
}

func TestIsServerFault(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"plain", errors.New("x"), true},
		{"not found", errUserNotFound, false},
		{"unavailable", apperr.ErrUnavailable, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := apperr.IsServerFault(tt.err); got != tt.want {
				t.Fatalf("IsServerFault = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWithFieldsCopies(t *testing.T) {
	err := apperr.ErrValidationFailed.WithFields(apperr.FieldViolation{Field: "email", Description: "required"})
	if len(apperr.ErrValidationFailed.Fields) != 0 {
		t.Fatal("WithFields must not mutate the sentinel")
	}
	if !errors.Is(err, apperr.ErrValidationFailed) || len(err.Fields) != 1 {
		t.Fatalf("unexpected %+v", err)
	}
}
