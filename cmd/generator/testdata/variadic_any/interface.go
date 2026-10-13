package variadic

import "context"

type Logger interface {
	Log(ctx context.Context, msg string, args ...any) error
	Put(ctx context.Context, v any, m map[string]interface{}) (any, error)
}
