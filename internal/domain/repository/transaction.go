package repository

import "context"

// TransactionManager runs fn inside a single transaction. The transaction is
// carried in the ctx passed to fn: repositories called with that ctx join it,
// while calls with any other ctx do not. fn's error rolls back and is
// returned unchanged; a nil return commits.
type TransactionManager interface {
	Execute(ctx context.Context, fn func(ctx context.Context) error) error
}
