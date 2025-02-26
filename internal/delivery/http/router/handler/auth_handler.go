package handler

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"server-template/internal/domain/usecase"

	"github.com/labstack/echo/v4"
)

type AuthHandler struct {
	authUseCase usecase.AuthHTTPUseCase
	logger      *slog.Logger
}

func NewAuthHandler(authUseCase usecase.AuthHTTPUseCase, logger *slog.Logger) *AuthHandler {
	return &AuthHandler{
		authUseCase: authUseCase,
		logger:      logger,
	}
}

// Request and response structs
type RegisterRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=6"`
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type UserResponse struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

type AuthResponse struct {
	Token string       `json:"token"`
	User  UserResponse `json:"user"`
}

// Register 處理用戶註冊請求
func (h *AuthHandler) Register(c echo.Context) error {
	var req RegisterRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Invalid request format",
		})
	}

	if err := c.Validate(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
	}

	// 調用 UseCase 層
	token, user, err := h.authUseCase.Register(c.Request().Context(), req.Email, req.Password)
	if err != nil {
		h.logger.Error("Failed to register user", slog.Any("error", err))
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
	}

	// 返回用戶信息和 token
	return c.JSON(http.StatusCreated, AuthResponse{
		Token: token,
		User: UserResponse{
			ID:        user.ID,
			Email:     user.Email,
			CreatedAt: user.CreatedAt,
		},
	})
}

// Login 處理用戶登入請求
func (h *AuthHandler) Login(c echo.Context) error {
	var req LoginRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Invalid request format",
		})
	}

	if err := c.Validate(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
	}

	// 調用 UseCase 層
	token, user, err := h.authUseCase.Login(c.Request().Context(), req.Email, req.Password)
	if err != nil {
		h.logger.Error("Failed to login user", slog.Any("error", err))
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"error": err.Error(),
		})
	}

	// 返回用戶信息和 token
	return c.JSON(http.StatusOK, AuthResponse{
		Token: token,
		User: UserResponse{
			ID:        user.ID,
			Email:     user.Email,
			CreatedAt: user.CreatedAt,
		},
	})
}

// Logout 處理用戶登出請求
func (h *AuthHandler) Logout(c echo.Context) error {
	// 從 Authorization 頭部獲取 token
	authHeader := c.Request().Header.Get("Authorization")
	if authHeader == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Authorization header is required",
		})
	}

	// 檢查 Bearer 前綴
	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Authorization header format must be Bearer {token}",
		})
	}

	tokenString := parts[1]

	// 調用 UseCase 層
	err := h.authUseCase.Logout(c.Request().Context(), tokenString)
	if err != nil {
		h.logger.Error("Failed to logout user", slog.Any("error", err))
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]string{
		"message": "Logged out successfully",
	})
}
