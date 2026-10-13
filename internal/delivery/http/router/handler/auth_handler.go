package handler

import (
	"net/http"
	"time"

	"server-template/internal/delivery/http/middleware"
	"server-template/internal/delivery/http/request"
	"server-template/internal/delivery/http/response"
	"server-template/internal/domain/entity"
	"server-template/internal/domain/usecase"
	"server-template/internal/pkg/apperr"

	"github.com/labstack/echo/v5"
)

// AuthHandler serves the auth endpoints. Handlers only return errors; the
// error middleware renders them.
type AuthHandler struct {
	authUseCase usecase.AuthHTTPUseCase
}

func NewAuthHandler(authUseCase usecase.AuthHTTPUseCase) *AuthHandler {
	return &AuthHandler{authUseCase: authUseCase}
}

// RegisterRequest password bounds mirror entity.MinPasswordLength/MaxPasswordLength;
// strength (upper, lower, digit) is enforced by the usecase.
type RegisterRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8,max=72"`
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,max=72"`
}

type UserResponse struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at,omitzero"`
}

type AuthResponse struct {
	Token string       `json:"token"`
	User  UserResponse `json:"user"`
}

type ProfileResponse struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
}

type LogoutResponse struct {
	Message string `json:"message"`
}

func newAuthResponse(token string, u *entity.User) AuthResponse {
	return AuthResponse{
		Token: token,
		User:  UserResponse{ID: u.ID, Email: u.Email, CreatedAt: u.CreatedAt},
	}
}

// Register handles POST /auth/register.
func (h *AuthHandler) Register(c *echo.Context) error {
	req, err := request.BindRequired[RegisterRequest](c)
	if err != nil {
		return err
	}

	token, u, err := h.authUseCase.Register(c.Request().Context(), req.Email, req.Password)
	if err != nil {
		return err
	}

	return response.Success(c, http.StatusCreated, newAuthResponse(token, u))
}

// Login handles POST /auth/login.
func (h *AuthHandler) Login(c *echo.Context) error {
	req, err := request.BindRequired[LoginRequest](c)
	if err != nil {
		return err
	}

	token, u, err := h.authUseCase.Login(c.Request().Context(), req.Email, req.Password)
	if err != nil {
		return err
	}

	return response.Success(c, http.StatusOK, newAuthResponse(token, u))
}

// Logout handles POST /auth/logout.
func (h *AuthHandler) Logout(c *echo.Context) error {
	token, err := middleware.BearerToken(c.Request())
	if err != nil {
		return err
	}

	if err := h.authUseCase.Logout(c.Request().Context(), token); err != nil {
		return err
	}

	return response.Success(c, http.StatusOK, LogoutResponse{Message: "logged out"})
}

// Profile handles GET /api/profile; it requires the JWT middleware.
func (h *AuthHandler) Profile(c *echo.Context) error {
	id, ok := middleware.IdentityFromContext(c.Request().Context())
	if !ok {
		return apperr.ErrUnauthenticated
	}

	return response.Success(c, http.StatusOK, ProfileResponse{UserID: id.UserID, Email: id.Email})
}
