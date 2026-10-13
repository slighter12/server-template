package router_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v5"

	"server-template/internal/delivery/http/middleware"
	"server-template/internal/delivery/http/router"
	"server-template/internal/delivery/http/validator"
	"server-template/internal/domain/entity"
	"server-template/internal/pkg/apperr"
	"server-template/proto/pb/authpb"
)

type fakeAuth struct {
	register func(email, password string) (string, *entity.User, error)
	login    func(email, password string) (string, *entity.User, error)
	logout   func(token string) error
	validate func(token string) (*authpb.ValidateTokenResponse, error)
}

func (f fakeAuth) Register(_ context.Context, email, password string) (string, *entity.User, error) {
	return f.register(email, password)
}

func (f fakeAuth) Login(_ context.Context, email, password string) (string, *entity.User, error) {
	return f.login(email, password)
}

func (f fakeAuth) Logout(_ context.Context, token string) error { return f.logout(token) }

func (f fakeAuth) ValidateToken(_ context.Context, token string) (*authpb.ValidateTokenResponse, error) {
	return f.validate(token)
}

type envelope struct {
	Data  json.RawMessage `json:"data"`
	Error *struct {
		Code   string `json:"code"`
		Fields []struct {
			Field string `json:"field"`
		} `json:"fields"`
	} `json:"error"`
}

func do(t *testing.T, uc fakeAuth, method, path, body, authz string) (*httptest.ResponseRecorder, envelope) {
	t.Helper()

	e := echo.New()
	e.Validator = validator.New()
	e.Use(middleware.HandleErrors())
	router.New(router.Params{AuthUC: uc}).Register(e)

	req := httptest.NewRequestWithContext(t.Context(), method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	}
	if authz != "" {
		req.Header.Set("Authorization", authz)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	var env envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode body %q: %v", rec.Body.String(), err)
	}

	return rec, env
}

func validToken(string) (*authpb.ValidateTokenResponse, error) {
	return authpb.ValidateTokenResponse_builder{
		Status: authpb.Status_builder{Code: new(int32(0))}.Build(),
		User:   authpb.User_builder{Id: new("u-1"), Email: new("a@b.com")}.Build(),
	}.Build(), nil
}

func TestRegister(t *testing.T) {
	t.Run("validation error lists fields", func(t *testing.T) {
		rec, env := do(t, fakeAuth{}, http.MethodPost, "/auth/register", `{"email":"nope","password":"x"}`, "")
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d", rec.Code)
		}
		if env.Error == nil || env.Error.Code != apperr.ErrValidationFailed.Code || len(env.Error.Fields) != 2 {
			t.Fatalf("error = %+v", env.Error)
		}
	})

	t.Run("email taken is 409", func(t *testing.T) {
		uc := fakeAuth{register: func(string, string) (string, *entity.User, error) {
			return "", nil, entity.ErrEmailTaken
		}}
		rec, env := do(t, uc, http.MethodPost, "/auth/register", `{"email":"a@b.com","password":"Passw0rdOK"}`, "")
		if rec.Code != http.StatusConflict || env.Error == nil || env.Error.Code != entity.ErrEmailTaken.Code {
			t.Fatalf("status = %d, error = %+v", rec.Code, env.Error)
		}
	})

	t.Run("success returns data envelope without password", func(t *testing.T) {
		uc := fakeAuth{register: func(email, _ string) (string, *entity.User, error) {
			return "tok", &entity.User{ID: "u-1", Email: email, Password: "secret-hash", CreatedAt: time.Unix(0, 0).UTC()}, nil
		}}
		rec, env := do(t, uc, http.MethodPost, "/auth/register", `{"email":"a@b.com","password":"Passw0rdOK"}`, "")
		if rec.Code != http.StatusCreated || env.Error != nil {
			t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
		}
		if strings.Contains(rec.Body.String(), "secret-hash") || !strings.Contains(string(env.Data), `"token":"tok"`) {
			t.Fatalf("data = %s", env.Data)
		}
	})
}

func TestLogin(t *testing.T) {
	uc := fakeAuth{login: func(string, string) (string, *entity.User, error) {
		return "", nil, entity.ErrInvalidCredentials
	}}
	rec, env := do(t, uc, http.MethodPost, "/auth/login", `{"email":"a@b.com","password":"whatever1"}`, "")
	if rec.Code != http.StatusUnauthorized || env.Error == nil || env.Error.Code != entity.ErrInvalidCredentials.Code {
		t.Fatalf("status = %d, error = %+v", rec.Code, env.Error)
	}

	rec, _ = do(t, fakeAuth{}, http.MethodPost, "/auth/login", ``, "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("empty body status = %d", rec.Code)
	}
}

func TestLogout(t *testing.T) {
	var got string
	uc := fakeAuth{logout: func(token string) error { got = token; return nil }}

	rec, _ := do(t, uc, http.MethodPost, "/auth/logout", "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no token status = %d", rec.Code)
	}

	rec, _ = do(t, uc, http.MethodPost, "/auth/logout", "", "Bearer abc")
	if rec.Code != http.StatusOK || got != "abc" {
		t.Fatalf("status = %d, token = %q", rec.Code, got)
	}
}

func TestProfile(t *testing.T) {
	uc := fakeAuth{validate: validToken}

	rec, env := do(t, uc, http.MethodGet, "/api/profile", "", "")
	if rec.Code != http.StatusUnauthorized || env.Error == nil || env.Error.Code != apperr.ErrUnauthenticated.Code {
		t.Fatalf("no token: status = %d, error = %+v", rec.Code, env.Error)
	}

	rec, env = do(t, uc, http.MethodGet, "/api/profile", "", "Basic abc")
	if rec.Code != http.StatusUnauthorized || env.Error.Code != apperr.ErrInvalidToken.Code {
		t.Fatalf("bad scheme: status = %d, error = %+v", rec.Code, env.Error)
	}

	bad := fakeAuth{validate: func(string) (*authpb.ValidateTokenResponse, error) { return nil, apperr.ErrInvalidToken }}
	rec, _ = do(t, bad, http.MethodGet, "/api/profile", "", "Bearer x")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("rejected token: status = %d", rec.Code)
	}

	rec, env = do(t, uc, http.MethodGet, "/api/profile", "", "Bearer good")
	if rec.Code != http.StatusOK || !strings.Contains(string(env.Data), `"user_id":"u-1"`) {
		t.Fatalf("ok: status = %d, body = %s", rec.Code, rec.Body)
	}
}
