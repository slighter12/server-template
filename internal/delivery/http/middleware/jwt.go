package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"server-template/internal/domain/usecase"

	"github.com/labstack/echo/v4"
)

type JWTConfig struct {
	AuthRPC usecase.AuthHTTPUseCase
	Logger  *slog.Logger
}

// JWT 返回一個 JWT 認證中間件
func JWT(config JWTConfig) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// 從 Authorization 頭部獲取 token
			authHeader := c.Request().Header.Get("Authorization")
			if authHeader == "" {
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"error": "Authorization header is required",
				})
			}

			// 檢查 Bearer 前綴
			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"error": "Authorization header format must be Bearer {token}",
				})
			}

			tokenString := parts[1]

			// 調用 gRPC 服務
			ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Second)
			defer cancel()

			resp, err := config.AuthRPC.ValidateToken(ctx, tokenString)
			if err != nil {
				config.Logger.Error("Failed to validate token", slog.Any("error", err))
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"error": "Invalid or expired token",
				})
			}

			// 檢查 gRPC 響應狀態
			if resp.GetStatus().GetCode() != int32(0) {
				return c.JSON(http.StatusUnauthorized, map[string]string{
					"error": resp.GetStatus().GetMessage(),
				})
			}

			// 將用戶信息存儲在上下文中，以便後續處理程序使用
			c.Set("user_id", resp.GetUser().GetId())
			c.Set("email", resp.GetUser().GetEmail())

			return next(c)
		}
	}
}
