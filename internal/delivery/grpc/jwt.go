package grpc

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"server-template/internal/pkg/apperr"

	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
)

const defaultAccessTokenTTL = 24 * time.Hour

// Claims 定義 JWT 的聲明
type Claims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

// tokenManager issues and verifies HS256 access tokens. Redis is mandatory: it
// stores issued tokens and the revocation list.
type tokenManager struct {
	secret []byte
	ttl    time.Duration
	redis  redis.Cmdable
	logger *slog.Logger
}

func newTokenManager(secret string, ttl time.Duration, rdb redis.Cmdable, logger *slog.Logger) *tokenManager {
	if ttl <= 0 {
		ttl = defaultAccessTokenTTL
	}

	return &tokenManager{secret: []byte(secret), ttl: ttl, redis: rdb, logger: logger}
}

func tokenKey(token string) string     { return "token:" + token }
func blacklistKey(token string) string { return "blacklist:" + token }

// issue signs a new token and records it in Redis.
func (m *tokenManager) issue(ctx context.Context, userID, email string) (string, error) {
	now := time.Now()
	claims := &Claims{
		UserID:    userID,
		Email:     email,
		ExpiresAt: jwt.NewNumericDate(now.Add(m.ttl)),
		IssuedAt:  jwt.NewNumericDate(now),
	}

	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
	if err != nil {
		return "", apperr.Wrap(fmt.Errorf("sign token: %w", err), apperr.ErrInternal)
	}

	if err := m.redis.Set(ctx, tokenKey(signed), userID, m.ttl).Err(); err != nil {
		return "", apperr.Wrap(fmt.Errorf("store token: %w", err), apperr.ErrUnavailable)
	}

	return signed, nil
}

// parse verifies signature, algorithm and expiry.
func (m *tokenManager) parse(tokenString string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(*jwt.Token) (any, error) {
		return m.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil {
		return nil, apperr.Wrap(err, apperr.ErrInvalidToken)
	}
	if !token.Valid {
		return nil, apperr.ErrInvalidToken
	}

	return claims, nil
}

// revoke blacklists a token for its remaining lifetime.
func (m *tokenManager) revoke(ctx context.Context, tokenString string) error {
	claims, err := m.parse(tokenString)
	if err != nil {
		return err
	}

	ttl := time.Until(claims.ExpiresAt.Time)
	if ttl <= 0 {
		return nil // already expired, nothing to blacklist
	}

	if err := m.redis.Set(ctx, blacklistKey(tokenString), "1", ttl).Err(); err != nil {
		return apperr.Wrap(fmt.Errorf("blacklist token: %w", err), apperr.ErrUnavailable)
	}
	if err := m.redis.Del(ctx, tokenKey(tokenString)).Err(); err != nil {
		m.logger.WarnContext(ctx, "failed to delete token from Redis", slog.Any("error", err))
	}

	return nil
}

// revoked reports whether the token is blacklisted.
func (m *tokenManager) revoked(ctx context.Context, tokenString string) (bool, error) {
	n, err := m.redis.Exists(ctx, blacklistKey(tokenString)).Result()
	if err != nil {
		return false, apperr.Wrap(fmt.Errorf("check blacklist: %w", err), apperr.ErrUnavailable)
	}

	return n > 0, nil
}
