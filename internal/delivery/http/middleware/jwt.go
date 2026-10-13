package middleware

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v5"

	"server-template/internal/domain/usecase"
	"server-template/internal/pkg/apperr"
)

const tokenValidationTimeout = 5 * time.Second

// Identity is the authenticated caller, stored in the request context by JWT.
type Identity struct {
	UserID string
	Email  string
}

type identityKey struct{}

// IdentityFromContext returns the caller authenticated by the JWT middleware.
func IdentityFromContext(ctx context.Context) (Identity, bool) {
	id, ok := ctx.Value(identityKey{}).(Identity)

	return id, ok
}

// BearerToken extracts the token from an "Authorization: Bearer <token>" header.
// A missing header is apperr.ErrUnauthenticated, a malformed one apperr.ErrInvalidToken.
func BearerToken(r *http.Request) (string, error) {
	h := r.Header.Get("Authorization")
	if h == "" {
		return "", apperr.ErrUnauthenticated
	}

	token, ok := strings.CutPrefix(h, "Bearer ")
	if token = strings.TrimSpace(token); !ok || token == "" {
		return "", apperr.ErrInvalidToken
	}

	return token, nil
}

// JWT authenticates requests by validating the bearer token through the auth
// service (gRPC loopback). Failures are returned as errors for the error middleware.
func JWT(authUC usecase.AuthHTTPUseCase) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			token, err := BearerToken(c.Request())
			if err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(c.Request().Context(), tokenValidationTimeout)
			defer cancel()

			resp, err := authUC.ValidateToken(ctx, token)
			if err != nil {
				return err
			}
			if resp.GetStatus().GetCode() != 0 || resp.GetUser().GetId() == "" {
				return apperr.ErrInvalidToken
			}

			ctx = context.WithValue(c.Request().Context(), identityKey{}, Identity{
				UserID: resp.GetUser().GetId(),
				Email:  resp.GetUser().GetEmail(),
			})
			c.SetRequest(c.Request().WithContext(ctx))

			return next(c)
		}
	}
}
