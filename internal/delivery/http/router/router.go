package router

import (
	"log/slog"
	"net/http"

	"server-template/config"
	"server-template/internal/delivery/http/middleware"
	"server-template/internal/delivery/http/router/handler"
	"server-template/internal/delivery/http/validator"
	"server-template/internal/domain/usecase"

	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	slogecho "github.com/samber/slog-echo"
)

// 也可以為 RegisterRoutes 創建一個 params 結構
type RouterParams struct {
	Router *echo.Echo
	Config *config.Config
	Logger *slog.Logger
	AuthUC usecase.AuthHTTPUseCase
}

func RegisterRoutes(params RouterParams) {
	// 設置驗證器
	params.Router.Validator = validator.New()

	// 中間件
	params.Router.Use(middleware.AltSvc(params.Config.HTTP.Port))
	params.Router.Use(slogecho.New(params.Logger))
	params.Router.Use(echomiddleware.Recover())
	params.Router.Use(echomiddleware.CORS())

	// 基本路由
	params.Router.GET("/ping", handlePing)
	params.Router.GET("/protocol", handleProtocol)

	// 創建處理程序
	authHandler := handler.NewAuthHandler(params.AuthUC, params.Logger)

	// 公開路由
	auth := params.Router.Group("/auth")
	auth.POST("/register", authHandler.Register)
	auth.POST("/login", authHandler.Login)
	auth.POST("/logout", authHandler.Logout)

	// 受保護的路由
	jwtConfig := middleware.JWTConfig{
		AuthRPC: params.AuthUC,
		Logger:  params.Logger,
	}
	api := params.Router.Group("/api")
	api.Use(middleware.JWT(jwtConfig))

	// 示例受保護的路由
	api.GET("/profile", func(c echo.Context) error {
		userID := c.Get("user_id").(string)
		email := c.Get("email").(string)

		return c.JSON(http.StatusOK, map[string]any{
			"user_id": userID,
			"email":   email,
		})
	})
}

func handlePing(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"message": "pong"})
}

func handleProtocol(c echo.Context) error {
	proto := "unknown"
	switch c.Request().ProtoMajor {
	case 3:
		proto = "HTTP/3"
	case 2:
		proto = "HTTP/2"
	case 1:
		proto = "HTTP/1.1"
	}

	return c.JSON(http.StatusOK, map[string]any{
		"protocol": proto,
		"headers":  c.Request().Header,
	})
}
