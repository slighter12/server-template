package grpc

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"strings"
	"testing"
	"time"

	"server-template/config"
	"server-template/internal/domain/entity"
	"server-template/internal/infrastructure/rpc"
	"server-template/internal/pkg/apperr"
	"server-template/proto/pb/authpb"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

type fakeAuth struct {
	registerErr error
	user        *entity.User
}

func (f *fakeAuth) Register(context.Context, string, string) (*entity.User, error) {
	return f.user, f.registerErr
}

func (f *fakeAuth) Login(context.Context, string, string) (*entity.User, error) {
	return f.user, f.registerErr
}

func (f *fakeAuth) GetUserByID(context.Context, string) (*entity.User, error) {
	return f.user, f.registerErr
}

func setup(t *testing.T, auth *fakeAuth) (authpb.AuthClient, *grpc.ClientConn) {
	t.Helper()

	cfg := &config.Config{}
	cfg.Auth.JWTSecret = "test-secret"
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	srv := newServer(auth, cfg, logger, rdb)
	lis := bufconn.Listen(1 << 20)
	go func() { _ = srv.serve(lis) }()
	t.Cleanup(func() { srv.stop(context.Background()) })

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(rpc.ErrorClientInterceptor()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	return authpb.NewAuthClient(conn), conn
}

func newUser() *entity.User {
	return &entity.User{ID: "u1", Email: "a@b.c", CreatedAt: time.Unix(1700000000, 0)}
}

func TestRegisterLoginPopulated(t *testing.T) {
	client, _ := setup(t, &fakeAuth{user: newUser()})
	ctx := t.Context()

	reg, err := client.Register(ctx, authpb.RegisterRequest_builder{Email: new("a@b.c"), Password: new("Passw0rdX")}.Build())
	require.NoError(t, err)
	assert.EqualValues(t, codes.OK, reg.GetStatus().GetCode())
	assert.Equal(t, "u1", reg.GetUser().GetId())
	assert.Equal(t, "a@b.c", reg.GetUser().GetEmail())
	assert.Equal(t, int64(1700000000), reg.GetUser().GetCreatedAt().GetSeconds())

	login, err := client.Login(ctx, authpb.LoginRequest_builder{Email: new("a@b.c"), Password: new("Passw0rdX")}.Build())
	require.NoError(t, err)
	assert.NotNil(t, login.GetStatus())
	assert.Equal(t, "u1", login.GetUser().GetId())
	assert.NotEmpty(t, login.GetToken())

	// Token works end to end against Redis.
	val, err := client.ValidateToken(ctx, authpb.ValidateTokenRequest_builder{Token: new(login.GetToken())}.Build())
	require.NoError(t, err)
	assert.Equal(t, "u1", val.GetUser().GetId())

	_, err = client.Logout(ctx, authpb.LogoutRequest_builder{Token: new(login.GetToken())}.Build())
	require.NoError(t, err)

	// The logged-out token is blacklisted.
	_, err = client.ValidateToken(ctx, authpb.ValidateTokenRequest_builder{Token: new(login.GetToken())}.Build())
	revoked, ok := errors.AsType[*apperr.Error](err)
	require.True(t, ok)
	assert.Equal(t, apperr.ErrInvalidToken.Code, revoked.Code)

	_, err = client.ValidateToken(ctx, authpb.ValidateTokenRequest_builder{Token: new("garbage")}.Build())
	ae, ok := errors.AsType[*apperr.Error](err)
	require.True(t, ok)
	assert.Equal(t, apperr.Unauthenticated, ae.Kind)
	assert.Equal(t, apperr.ErrInvalidToken.Code, ae.Code)
}

func TestErrorRoundTrip(t *testing.T) {
	client, _ := setup(t, &fakeAuth{registerErr: entity.ErrEmailTaken})

	_, err := client.Register(t.Context(), authpb.RegisterRequest_builder{Email: new("a@b.c"), Password: new("x")}.Build())
	require.Error(t, err)

	ae, ok := errors.AsType[*apperr.Error](err)
	require.True(t, ok, "client must return *apperr.Error, got %T", err)
	assert.Equal(t, "EMAIL_TAKEN", ae.Code)
	assert.Equal(t, apperr.Conflict, ae.Kind)
	assert.Equal(t, entity.ErrEmailTaken.Message, ae.Message)
	assert.ErrorIs(t, err, entity.ErrEmailTaken)
	assert.Equal(t, codes.AlreadyExists, status.Code(errors.Unwrap(ae)))
}

func TestServerFaultIsGeneric(t *testing.T) {
	secret := "dial tcp 10.0.0.1:5432: password=hunter2"
	client, _ := setup(t, &fakeAuth{registerErr: errors.New(secret)})

	_, err := client.Register(t.Context(), authpb.RegisterRequest_builder{Email: new("a@b.c"), Password: new("x")}.Build())
	require.Error(t, err)

	ae, ok := errors.AsType[*apperr.Error](err)
	require.True(t, ok)
	assert.Equal(t, apperr.Internal, ae.Kind)
	assert.Equal(t, apperr.ErrInternal.Code, ae.Code)
	assert.Equal(t, apperr.ErrInternal.Message, ae.Message)

	raw, _ := status.FromError(errors.Unwrap(ae))
	assert.False(t, strings.Contains(raw.Message(), "hunter2"))
	assert.Equal(t, codes.Internal, raw.Code())
}

func TestKindCodeTablesAreInverse(t *testing.T) {
	for kind := apperr.Internal; kind <= apperr.Unavailable; kind++ {
		err := rpc.FromStatusError(ToStatusError(apperr.New(kind, "X", "m")))
		assert.Equal(t, kind, apperr.KindOf(err), kind.String())
	}
}

func TestHealthServing(t *testing.T) {
	_, conn := setup(t, &fakeAuth{user: newUser()})
	hc := grpc_health_v1.NewHealthClient(conn)

	require.Eventually(t, func() bool {
		resp, err := hc.Check(t.Context(), &grpc_health_v1.HealthCheckRequest{})

		return err == nil && resp.GetStatus() == grpc_health_v1.HealthCheckResponse_SERVING
	}, 2*time.Second, 10*time.Millisecond)
}
