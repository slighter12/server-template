package http

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"log/slog"
	"math/big"
	"net"
	"net/http"
	"time"

	"server-template/config"
	"server-template/internal/domain/delivery"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/pkg/errors"
	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
	slogecho "github.com/samber/slog-echo"
	"go.uber.org/fx"
)

type httpServer struct {
	fx.Lifecycle

	cfg      *config.Config
	logger   *slog.Logger
	h3Server *http3.Server
}

func NewHTTP(lc fx.Lifecycle, cfg *config.Config, logger *slog.Logger) delivery.Delivery {
	return &httpServer{
		Lifecycle: lc,
		cfg:       cfg,
		logger:    logger,
	}
}

func (s *httpServer) Serve(ctx context.Context) error {
	echoServer := echo.New()
	s.registerRoutes(echoServer)

	tlsCert, err := generateTLSConfig()
	if err != nil {
		return errors.Wrap(err, "generate TLS config")
	}

	errChan := make(chan error, 2)

	// 啟動 HTTP/3 服務器
	go func() {
		if err := s.startHTTP3Server(echoServer, tlsCert); err != nil {
			errChan <- errors.Wrap(err, "failed to start http3 server")

			return
		}
	}()

	// 啟動 HTTPS (HTTP/2) 服務器
	go func() {
		if err := s.startHTTPServer(echoServer, tlsCert); err != nil {
			errChan <- errors.Wrap(err, "failed to start https server")

			return
		}
	}()

	// 等待 context 取消或者服務器錯誤
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		return errors.WithStack(echoServer.Shutdown(shutdownCtx))
	case err := <-errChan:
		return err
	}
}

func (s *httpServer) registerRoutes(router *echo.Echo) {
	// Add the SetQUICHeaders middleware
	router.Use(s.setQUICHeadersMiddleware)

	// Middleware to set Alt-Svc header
	router.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Response().Header().Set("Alt-Svc", fmt.Sprintf(`h3=":%d"`, s.cfg.HTTP.Port))

			return next(c)
		}
	})

	router.Use(slogecho.New(s.logger))
	router.Use(middleware.Recover())

	router.GET("/ping", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"message": "pong"})
	})

	router.GET("/protocol", func(c echo.Context) error {
		proto := "unknown"
		switch c.Request().ProtoMajor {
		case 3:
			proto = "HTTP/3"
		case 2:
			proto = "HTTP/2"
		case 1:
			proto = "HTTP/1.1"
		}

		return c.JSON(http.StatusOK, map[string]interface{}{
			"protocol": proto,
			"headers":  c.Request().Header,
		})
	})
}

func (s *httpServer) setQUICHeadersMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		if c.Request().ProtoMajor < 3 {
			err := s.h3Server.SetQUICHeaders(c.Response().Header())
			if err != nil {
				s.logger.Error("Failed to set QUIC headers", slog.Any("error", err))
				// Decide how to handle the error.  For example:
				// return echo.NewHTTPError(http.StatusInternalServerError, "Failed to set QUIC headers")
			}
		}

		wasPotentiallyReplayed := !c.Request().TLS.HandshakeComplete
		slog.Info("Was the request potentially replayed?", slog.Int("ProtoMajor", c.Request().ProtoMajor), slog.Bool("wasPotentiallyReplayed", wasPotentiallyReplayed))

		return next(c)
	}
}

// 定義啟動 HTTP/3 服務器的方法
func (s *httpServer) startHTTP3Server(echoServer *echo.Echo, tlsCert *tls.Certificate) error {
	udpAddr := &net.UDPAddr{Port: s.cfg.HTTP.Port}
	udpConn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return errors.Wrap(err, "failed to create UDP listener")
	}

	server := &http3.Server{
		Addr:    fmt.Sprintf(":%d", s.cfg.HTTP.Port),
		Handler: echoServer,
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{*tlsCert},
			MinVersion:   tls.VersionTLS12,
			NextProtos:   []string{"h3", "h3-29"}, // 簡化支持的協議
		},
		QUICConfig: &quic.Config{
			MaxIdleTimeout:             30 * time.Second,
			KeepAlivePeriod:            10 * time.Second,
			EnableDatagrams:            true,
			MaxIncomingStreams:         1000,
			MaxStreamReceiveWindow:     10 * 1024 * 1024,
			MaxConnectionReceiveWindow: 15 * 1024 * 1024,
			Allow0RTT:                  true,
		},
	}

	s.h3Server = server

	slog.Info("Starting HTTP/3 server", slog.Any("port", s.cfg.HTTP.Port))

	return server.Serve(udpConn)
}

// 定義啟動 HTTPS (HTTP/2) 服務器的方法
func (s *httpServer) startHTTPServer(echoServer *echo.Echo, tlsCert *tls.Certificate) error {
	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", s.cfg.HTTP.Port),
		Handler:           echoServer,
		ReadHeaderTimeout: 20 * time.Second,
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{*tlsCert},
			MinVersion:   tls.VersionTLS12,
			NextProtos:   []string{"h2", "h3", "h3-29", "quic"}, // 添加 HTTP/3 支援
		},
	}

	slog.Info("Starting HTTPS server", slog.Any("port", s.cfg.HTTP.Port))
	if err := server.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
		return errors.Wrap(err, "failed to serve https")
	}

	return nil
}

func generateTLSConfig() (*tls.Certificate, error) {
	// Generate private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, errors.Wrap(err, "generate private key")
	}

	// Create certificate template
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Development"},
			CommonName:   "localhost",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
		DNSNames:              []string{"localhost"},
	}

	// Create certificate
	derBytes, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return nil, errors.Wrap(err, "create certificate")
	}

	// Create TLS certificate
	cert := &tls.Certificate{
		Certificate: [][]byte{derBytes},
		PrivateKey:  privateKey,
	}

	return cert, nil
}
