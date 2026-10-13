package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"server-template/internal/pkg/apperr"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// fakePool is a hermetic gorm ConnPool that records transaction control calls.
type fakePool struct {
	beginErr  error
	commitErr error
	begins    int
	commits   int
	rollbacks int
}

func (p *fakePool) PrepareContext(context.Context, string) (*sql.Stmt, error) { return nil, nil }
func (p *fakePool) ExecContext(context.Context, string, ...any) (sql.Result, error) {
	return nil, nil
}
func (p *fakePool) QueryContext(context.Context, string, ...any) (*sql.Rows, error) {
	return nil, nil
}
func (p *fakePool) QueryRowContext(context.Context, string, ...any) *sql.Row { return nil }

func (p *fakePool) BeginTx(context.Context, *sql.TxOptions) (gorm.ConnPool, error) {
	p.begins++
	if p.beginErr != nil {
		return nil, p.beginErr
	}

	return &fakeTx{fakePool: p}, nil
}

type fakeTx struct{ *fakePool }

func (t *fakeTx) Commit() error {
	t.commits++

	return t.commitErr
}

func (t *fakeTx) Rollback() error {
	t.rollbacks++

	return nil
}

func newFakeDB(t *testing.T, pool *fakePool) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(gormpostgres.New(gormpostgres.Config{Conn: pool}), &gorm.Config{
		DisableAutomaticPing:   true,
		SkipDefaultTransaction: true,
	})
	require.NoError(t, err)

	return db
}

func TestDBFromContext(t *testing.T) {
	fallback := &gorm.DB{}
	assert.Same(t, fallback, DBFromContext(context.Background(), fallback))

	tx := &gorm.DB{}
	ctx := context.WithValue(context.Background(), txKey{}, tx)
	assert.Same(t, tx, DBFromContext(ctx, fallback))
}

func TestExecute_CommitsAndCarriesTxInContext(t *testing.T) {
	pool := &fakePool{}
	db := newFakeDB(t, pool)
	tm := NewTransactionManager(db, nil)

	err := tm.Execute(t.Context(), func(ctx context.Context) error {
		assert.NotSame(t, db, DBFromContext(ctx, db))

		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, 1, pool.commits)
	assert.Zero(t, pool.rollbacks)
}

func TestExecute_RollsBackOnError(t *testing.T) {
	pool := &fakePool{}
	tm := NewTransactionManager(newFakeDB(t, pool), nil)
	sentinel := errors.New("boom")

	err := tm.Execute(t.Context(), func(context.Context) error { return sentinel })
	assert.Same(t, sentinel, err)
	assert.Equal(t, 1, pool.rollbacks)
	assert.Zero(t, pool.commits)
}

func TestExecute_RollsBackOnPanic(t *testing.T) {
	pool := &fakePool{}
	tm := NewTransactionManager(newFakeDB(t, pool), nil)

	assert.PanicsWithValue(t, "oops", func() {
		_ = tm.Execute(t.Context(), func(context.Context) error { panic("oops") })
	})
	assert.Equal(t, 1, pool.rollbacks)
}

func TestExecute_CommitFailureIsInternal(t *testing.T) {
	pool := &fakePool{commitErr: errors.New("commit failed")}
	tm := NewTransactionManager(newFakeDB(t, pool), nil)

	err := tm.Execute(t.Context(), func(context.Context) error { return nil })
	require.ErrorIs(t, err, apperr.ErrInternal)
}

func TestExecute_BeginFailureIsInternal(t *testing.T) {
	pool := &fakePool{beginErr: errors.New("no conn")}
	tm := NewTransactionManager(newFakeDB(t, pool), nil)

	err := tm.Execute(t.Context(), func(context.Context) error {
		t.Fatal("fn must not run")

		return nil
	})
	require.ErrorIs(t, err, apperr.ErrInternal)
}

func TestExecute_NestedReusesOuterTransaction(t *testing.T) {
	pool := &fakePool{}
	tm := NewTransactionManager(newFakeDB(t, pool), nil)

	err := tm.Execute(t.Context(), func(ctx context.Context) error {
		return tm.Execute(ctx, func(context.Context) error { return nil })
	})
	require.NoError(t, err)
	assert.Equal(t, 1, pool.begins)
	assert.Equal(t, 1, pool.commits)
}
