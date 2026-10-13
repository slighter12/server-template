package rpc

import (
	"context"

	"server-template/internal/pkg/apperr"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// codeToKind is the inverse of the server-side Kind -> codes.Code table.
var codeToKind = map[codes.Code]apperr.Kind{
	codes.InvalidArgument:    apperr.InvalidArgument,
	codes.OutOfRange:         apperr.InvalidArgument,
	codes.NotFound:           apperr.NotFound,
	codes.AlreadyExists:      apperr.Conflict,
	codes.Aborted:            apperr.Conflict,
	codes.Unauthenticated:    apperr.Unauthenticated,
	codes.PermissionDenied:   apperr.PermissionDenied,
	codes.FailedPrecondition: apperr.FailedPrecondition,
	codes.ResourceExhausted:  apperr.TooManyRequests,
	codes.Unavailable:        apperr.Unavailable,
	codes.DeadlineExceeded:   apperr.Unavailable,
	codes.Canceled:           apperr.Unavailable,
	codes.Internal:           apperr.Internal,
}

// fallbackSentinels provide a code when the peer sent no ErrorInfo.
var fallbackSentinels = map[apperr.Kind]*apperr.Error{
	apperr.InvalidArgument:    apperr.ErrInvalidInput,
	apperr.NotFound:           apperr.ErrNotFound,
	apperr.Conflict:           apperr.ErrConflict,
	apperr.Unauthenticated:    apperr.ErrUnauthenticated,
	apperr.PermissionDenied:   apperr.ErrForbidden,
	apperr.TooManyRequests:    apperr.ErrTooManyRequests,
	apperr.Unavailable:        apperr.ErrUnavailable,
	apperr.Internal:           apperr.ErrInternal,
	apperr.FailedPrecondition: apperr.New(apperr.FailedPrecondition, "FAILED_PRECONDITION", "failed precondition"),
}

// FromStatusError converts a gRPC status error back into *apperr.Error.
// The original status error is kept as the cause. A nil error returns nil.
func FromStatusError(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := err.(*apperr.Error); ok { //nolint:errorlint // already converted
		return err
	}

	st, ok := status.FromError(err)
	if !ok {
		return apperr.Wrap(err, apperr.ErrUnavailable)
	}

	kind, known := codeToKind[st.Code()]
	if !known {
		kind = apperr.Internal
	}

	code := fallbackSentinels[kind].Code
	for _, d := range st.Details() {
		if info, ok := d.(*errdetails.ErrorInfo); ok && info.GetReason() != "" {
			code = info.GetReason()

			break
		}
	}

	msg := st.Message()
	if kind.IsServerFault() || msg == "" {
		msg = fallbackSentinels[kind].Message
	}

	return apperr.Wrap(err, apperr.New(kind, code, msg))
}

// ErrorClientInterceptor makes every unary call return *apperr.Error so the
// original error code and kind survive an in-process or cross-service hop.
func ErrorClientInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		return FromStatusError(invoker(ctx, method, req, reply, cc, opts...))
	}
}
