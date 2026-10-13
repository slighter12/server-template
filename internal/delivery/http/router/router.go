// Package router registers the application routes on the shared Echo instance.
// Cross-cutting middleware (recover, request id, logging, CORS, error
// rendering) belongs to the server builder, not here.
package router

import (
	"server-template/internal/delivery/http/middleware"
	"server-template/internal/delivery/http/router/handler"
	"server-template/internal/domain/usecase"

	"github.com/labstack/echo/v5"
	"go.uber.org/fx"
)

// Params are the router dependencies.
type Params struct {
	fx.In

	AuthUC usecase.AuthHTTPUseCase
}

// Router registers the auth routes. Its Register method satisfies the server
// package's RouteRegistrar interface.
type Router struct {
	authUC usecase.AuthHTTPUseCase
	auth   *handler.AuthHandler
}

func New(params Params) *Router {
	return &Router{
		authUC: params.AuthUC,
		auth:   handler.NewAuthHandler(params.AuthUC),
	}
}

// Register adds the routes to e.
func (r *Router) Register(e *echo.Echo) {
	auth := e.Group("/auth")
	auth.POST("/register", r.auth.Register)
	auth.POST("/login", r.auth.Login)
	auth.POST("/logout", r.auth.Logout)

	api := e.Group("/api", middleware.JWT(r.authUC))
	api.GET("/profile", r.auth.Profile)
}
