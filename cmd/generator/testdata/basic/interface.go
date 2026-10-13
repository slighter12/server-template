package basic

import (
	"context"

	"server-template/internal/domain/entity"
)

type Store interface {
	// Create returns only an error.
	Create(ctx context.Context, u *entity.User) error
	// Find returns a value and an error.
	Find(ctx context.Context, id string) (*entity.User, error)
	// Pair has multiple results.
	Pair(ctx context.Context, a, b int) (string, int, error)
	// Count has no error result.
	Count(ctx context.Context) int
	// Touch has no results at all.
	Touch(ctx context.Context)
}
