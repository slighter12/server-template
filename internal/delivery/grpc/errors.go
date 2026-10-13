package grpc

import (
	"context"
	"errors"
	"log/slog"

	"server-template/internal/pkg/apperr"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// errorDomain identifies this service in errdetails.ErrorInfo.
const errorDomain = "server-template"

// kindToCode is the only place where apperr.Kind is mapped to a gRPC code.
var kindToCode = map[apperr.Kind]codes.Code{
	apperr.InvalidArgument:    codes.InvalidArgument,
	apperr.NotFound:           codes.NotFound,
	apperr.Conflict:           codes.AlreadyExists,
	apperr.Unauthenticated:    codes.Unauthenticated,
	apperr.PermissionDenied:   codes.PermissionDenied,
	apperr.FailedPrecondition: codes.FailedPrecondition,
	apperr.TooManyRequests:    codes.ResourceExhausted,
	apperr.Unavailable:        codes.Unavailable,
	apperr.Internal:           codes.Internal,
}

// CodeOf returns the gRPC code for a Kind (Internal for unknown kinds).
func CodeOf(k apperr.Kind) codes.Code {
	if c, ok := kindToCode[k]; ok {
		return c
	}

	return codes.Internal
}

// ToStatusError converts err into a gRPC status error. The status message and
// ErrorInfo are client-safe; server faults never expose message or cause.
func ToStatusError(err error) error {
	ae := apperr.From(err)

	msg := ae.Message
	switch ae.Kind { //nolint:exhaustive // only server faults need a generic message
	case apperr.Internal:
		msg = apperr.ErrInternal.Message
	case apperr.Unavailable:
		msg = apperr.ErrUnavailable.Message
	}

	st := status.New(CodeOf(ae.Kind), msg)
	if withDetails, derr := st.WithDetails(&errdetails.ErrorInfo{Reason: ae.Code, Domain: errorDomain}); derr == nil {
		st = withDetails
	}

	return st.Err()
}

// ErrorInterceptor lets handlers simply return errors: it logs server faults
// (with stack) and converts everything into a gRPC status error.
func ErrorInterceptor(logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		resp, err := handler(ctx, req)
		if err == nil {
			return resp, nil
		}
		ae, isApp := errors.AsType[*apperr.Error](err)
		// A status error that is not ours (e.g. from a nested call) passes through.
		if !isApp {
			if _, ok := status.FromError(err); ok {
				return resp, err
			}
		}

		if apperr.IsServerFault(err) {
			attrs := []any{slog.String("method", info.FullMethod), slog.Any("error", err)}
			if isApp && ae.Stack() != "" {
				attrs = append(attrs, slog.String("stack", ae.Stack()))
			}
			logger.ErrorContext(ctx, "gRPC server fault", attrs...)
		}

		return nil, ToStatusError(err)
	}
}
