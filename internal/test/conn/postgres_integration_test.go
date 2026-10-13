//go:build integration

package conn_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"server-template/internal/domain/entity"
	"server-template/internal/domain/entity/user"
	"server-template/internal/repository"

	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const postgresMigrationsDir = "database/migrations/postgres"

// newGoose builds a goose provider over the real migration files. The working
// directory is the repo root (see TestMain).
func newGoose(sqlDB *sql.DB) (*goose.Provider, error) {
	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, os.DirFS(postgresMigrationsDir))
	if err != nil {
		return nil, fmt.Errorf("new goose provider: %w", err)
	}

	return provider, nil
}

// migratePostgres applies every pending goose migration. It is idempotent: an
// already migrated database is a no-op (goose records versions in
// goose_db_version).
func migratePostgres(ctx context.Context) error {
	sqlDB, err := d.Postgres.DB()
	if err != nil {
		return fmt.Errorf("postgres sql.DB: %w", err)
	}

	provider, err := newGoose(sqlDB)
	if err != nil {
		return err
	}
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}

	return nil
}

func TestPostgresMigrations(t *testing.T) {
	t.Parallel()

	sqlDB, err := d.Postgres.DB()
	require.NoError(t, err)

	provider, err := newGoose(sqlDB)
	require.NoError(t, err)

	// Already applied by TestMain: running again must be a no-op.
	results, err := provider.Up(t.Context())
	require.NoError(t, err)
	assert.Empty(t, results, "second Up must not apply anything")

	current, err := provider.GetDBVersion(t.Context())
	require.NoError(t, err)
	sources := provider.ListSources()
	require.NotEmpty(t, sources)
	assert.Equal(t, sources[len(sources)-1].Version, current, "database is at the latest migration")

	var idx string
	require.NoError(t, d.Postgres.WithContext(t.Context()).
		Raw("SELECT indexdef FROM pg_indexes WHERE tablename = 'users' AND indexname = 'udx_users_email_lower'").
		Scan(&idx).Error)
	assert.Contains(t, idx, "lower(", "expression index from 20261013000000_alter_users.sql exists")
}

func newUser(t *testing.T) *entity.User {
	t.Helper()

	return &entity.User{
		Name:     "it",
		Email:    uniq("it") + "@Example.com",
		Password: "$argon2id$v=19$m=65536,t=3,p=2$c29tZXNhbHQ$aGFzaA", // opaque encoded hash
	}
}

func deleteUserAfter(t *testing.T, email string) {
	t.Helper()

	t.Cleanup(func() {
		d.Postgres.WithContext(cleanupCtx(t)).Exec("DELETE FROM users WHERE lower(email) = lower(?)", email)
	})
}

func TestPostgresUserRepository(t *testing.T) {
	t.Parallel()

	repo, err := repository.NewUserRepository(d.Postgres)
	require.NoError(t, err)

	t.Run("create find by email and id", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()
		u := newUser(t)
		u.Email = entity.NormalizeEmail(u.Email)
		deleteUserAfter(t, u.Email)

		require.NoError(t, repo.Create(ctx, u))
		assert.NotEmpty(t, u.ID, "gen_random_uuid() default is read back (RETURNING)")
		assert.False(t, u.CreatedAt.IsZero())

		byEmail, err := repo.FindByEmail(ctx, u.Email)
		require.NoError(t, err)
		assert.Equal(t, u.ID, byEmail.ID)
		assert.Equal(t, u.Email, byEmail.Email)
		assert.Equal(t, u.Password, byEmail.Password)
		assert.Equal(t, user.UserStatusActive, byEmail.Status, "status column default")
		assert.WithinDuration(t, u.CreatedAt, byEmail.CreatedAt, time.Second)

		byID, err := repo.FindByID(ctx, u.ID)
		require.NoError(t, err)
		assert.Equal(t, u.Email, byID.Email)
	})

	t.Run("duplicate email differing in case is ErrEmailTaken", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()
		first := newUser(t)
		first.Email = strings.ToLower(first.Email)
		deleteUserAfter(t, first.Email)
		require.NoError(t, repo.Create(ctx, first))

		second := newUser(t)
		second.Email = strings.ToUpper(first.Email) // bypasses NormalizeEmail on purpose: udx_users_email_lower must catch it
		err := repo.Create(ctx, second)
		require.Error(t, err)
		assert.ErrorIs(t, err, entity.ErrEmailTaken)
	})

	t.Run("missing user is ErrUserNotFound", func(t *testing.T) {
		t.Parallel()

		_, err := repo.FindByEmail(t.Context(), uniq("missing")+"@example.com")
		assert.ErrorIs(t, err, entity.ErrUserNotFound)

		_, err = repo.FindByID(t.Context(), "00000000-0000-0000-0000-000000000000")
		assert.ErrorIs(t, err, entity.ErrUserNotFound)
	})
}

func TestPostgresTransactionManager(t *testing.T) {
	t.Parallel()

	repo, err := repository.NewUserRepository(d.Postgres)
	require.NoError(t, err)
	tm, err := repository.NewTransactionManager(d.Postgres, nil)
	require.NoError(t, err)

	t.Run("commit persists", func(t *testing.T) {
		t.Parallel()

		u := newUser(t)
		u.Email = entity.NormalizeEmail(u.Email)
		deleteUserAfter(t, u.Email)

		require.NoError(t, tm.Execute(t.Context(), func(ctx context.Context) error {
			return repo.Create(ctx, u)
		}))

		got, err := repo.FindByEmail(t.Context(), u.Email)
		require.NoError(t, err)
		assert.Equal(t, u.ID, got.ID)
	})

	t.Run("error rolls back", func(t *testing.T) {
		t.Parallel()

		u := newUser(t)
		u.Email = entity.NormalizeEmail(u.Email)
		deleteUserAfter(t, u.Email)
		boom := errors.New("boom")

		err := tm.Execute(t.Context(), func(ctx context.Context) error {
			if err := repo.Create(ctx, u); err != nil {
				return err
			}
			// Visible inside the transaction...
			if _, err := repo.FindByEmail(ctx, u.Email); err != nil {
				return err
			}

			return boom
		})
		require.ErrorIs(t, err, boom)

		// ...and gone afterwards.
		_, err = repo.FindByEmail(t.Context(), u.Email)
		assert.ErrorIs(t, err, entity.ErrUserNotFound)
	})

	t.Run("nested Execute reuses the transaction", func(t *testing.T) {
		t.Parallel()

		u := newUser(t)
		u.Email = entity.NormalizeEmail(u.Email)
		deleteUserAfter(t, u.Email)
		boom := errors.New("outer fails after nested succeeded")

		err := tm.Execute(t.Context(), func(outer context.Context) error {
			outerTx := repository.DBFromContext(outer, nil)
			require.NotNil(t, outerTx, "outer ctx carries the transaction")

			if err := tm.Execute(outer, func(inner context.Context) error {
				assert.Same(t, outerTx, repository.DBFromContext(inner, nil), "nested call joins the outer transaction")

				return repo.Create(inner, u)
			}); err != nil {
				return err
			}

			return boom
		})
		require.ErrorIs(t, err, boom)

		// The nested "commit" did not commit: the outer rollback removed the row.
		_, err = repo.FindByEmail(t.Context(), u.Email)
		assert.ErrorIs(t, err, entity.ErrUserNotFound)
	})

	t.Run("nested Execute commits with the outer", func(t *testing.T) {
		t.Parallel()

		u := newUser(t)
		u.Email = entity.NormalizeEmail(u.Email)
		deleteUserAfter(t, u.Email)

		require.NoError(t, tm.Execute(t.Context(), func(outer context.Context) error {
			return tm.Execute(outer, func(inner context.Context) error {
				return repo.Create(inner, u)
			})
		}))

		_, err := repo.FindByEmail(t.Context(), u.Email)
		assert.NoError(t, err)
	})
}

// pgThing is a scratch model; the table name is set per test with db.Table.
type pgThing struct {
	ID        string    `gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	Key       string    `gorm:"type:text;not null;uniqueIndex"`
	Body      string    `gorm:"type:text;not null;default:''"`
	Hits      int       `gorm:"type:integer;not null;default:0"`
	At        time.Time `gorm:"type:timestamptz;not null"`
	CreatedAt time.Time `gorm:"type:timestamptz;not null;default:now()"`
}

func pgScratchTable(t *testing.T) *gorm.DB {
	t.Helper()

	table := uniq("it_pg")
	db := d.Postgres.WithContext(t.Context()).Table(table).Session(&gorm.Session{}) // Session: every chain call clones the statement
	require.NoError(t, db.AutoMigrate(&pgThing{}))
	t.Cleanup(func() { d.Postgres.WithContext(cleanupCtx(t)).Exec(fmt.Sprintf("DROP TABLE IF EXISTS %q", table)) })

	return db
}

func TestPostgresGORMSyntax(t *testing.T) {
	t.Parallel()

	var version string
	require.NoError(t, d.Postgres.WithContext(t.Context()).Raw("SHOW server_version").Scan(&version).Error)
	t.Logf("postgres server_version=%s", version)

	t.Run("round trip text uuid timestamptz and defaults", func(t *testing.T) {
		t.Parallel()

		db := pgScratchTable(t)
		// Non-UTC offset, microsecond precision (the timestamptz resolution).
		at := time.Date(2026, 10, 13, 9, 8, 7, 123456000, time.FixedZone("UTC+8", 8*3600))
		body := "héllo 世界 \U0001F680 'quoted' \"double\" \\ back"
		row := pgThing{Key: "k1", Body: body, At: at}

		require.NoError(t, db.Create(&row).Error)
		assert.Len(t, row.ID, 36, "gen_random_uuid() default came back via RETURNING")

		var got pgThing
		require.NoError(t, db.Where("id = ?", row.ID).Take(&got).Error)
		assert.Equal(t, body, got.Body)
		assert.True(t, at.Equal(got.At), "timestamptz instant preserved: want %s got %s", at, got.At)
		assert.False(t, got.CreatedAt.IsZero(), "now() default")
		assert.Equal(t, 0, got.Hits)

		var typ string
		require.NoError(t, d.Postgres.WithContext(t.Context()).
			Raw("SELECT pg_typeof(gen_random_uuid())::text").Scan(&typ).Error)
		assert.Equal(t, "uuid", typ)
	})

	t.Run("RETURNING", func(t *testing.T) {
		t.Parallel()

		db := pgScratchTable(t)
		rows := []pgThing{
			{Key: "a", Hits: 1, At: time.Now()},
			{Key: "b", Hits: 2, At: time.Now()},
		}
		require.NoError(t, db.Create(&rows).Error)

		// UPDATE ... RETURNING through the gorm clause.
		var updated []pgThing
		res := db.Model(&updated).Clauses(clause.Returning{}).
			Where("hits >= ?", 1).Update("hits", gorm.Expr("hits + 10"))
		require.NoError(t, res.Error)
		require.EqualValues(t, 2, res.RowsAffected)
		hits := []int{updated[0].Hits, updated[1].Hits}
		assert.ElementsMatch(t, []int{11, 12}, hits)

		// Raw INSERT ... RETURNING with several columns.
		var ret struct {
			ID   string
			Body string
		}
		require.NoError(t, d.Postgres.WithContext(t.Context()).
			Raw(fmt.Sprintf(`INSERT INTO %q (key, body, at) VALUES ('c', 'raw', now()) RETURNING id, body`, db.Statement.Table)).
			Scan(&ret).Error)
		assert.Len(t, ret.ID, 36)
		assert.Equal(t, "raw", ret.Body)

		// DELETE ... RETURNING.
		var deleted []pgThing
		require.NoError(t, db.Clauses(clause.Returning{}).Where("key = ?", "a").Delete(&deleted).Error)
		require.Len(t, deleted, 1)
		assert.Equal(t, "a", deleted[0].Key)
	})

	t.Run("ON CONFLICT upsert", func(t *testing.T) {
		t.Parallel()

		db := pgScratchTable(t)
		require.NoError(t, db.Create(&pgThing{Key: "u", Body: "first", Hits: 1, At: time.Now()}).Error)

		upsert := clause.OnConflict{
			Columns:   []clause.Column{{Name: "key"}},
			DoUpdates: clause.AssignmentColumns([]string{"body"}),
		}
		require.NoError(t, db.Clauses(upsert).Create(&pgThing{Key: "u", Body: "second", Hits: 99, At: time.Now()}).Error)

		var got []pgThing
		require.NoError(t, db.Find(&got).Error)
		require.Len(t, got, 1)
		assert.Equal(t, "second", got[0].Body, "DO UPDATE SET body = excluded.body")
		assert.Equal(t, 1, got[0].Hits, "hits was not in DoUpdates")

		// DO NOTHING leaves the row alone and does not error.
		require.NoError(t, db.Clauses(clause.OnConflict{DoNothing: true}).
			Create(&pgThing{Key: "u", Body: "third", At: time.Now()}).Error)
		require.NoError(t, db.Find(&got).Error)
		assert.Equal(t, "second", got[0].Body)

		// Atomic counter upsert with an expression (the target column must be table-qualified).
		counter := clause.OnConflict{
			Columns:   []clause.Column{{Name: "key"}},
			DoUpdates: clause.Assignments(map[string]any{"hits": gorm.Expr(fmt.Sprintf(`%q."hits" + excluded."hits"`, db.Statement.Table))}),
		}
		for range 3 {
			require.NoError(t, db.Clauses(counter).Create(&pgThing{Key: "u", Hits: 5, At: time.Now()}).Error)
		}
		var hits int
		require.NoError(t, db.Select("hits").Where("key = ?", "u").Scan(&hits).Error)
		assert.Equal(t, 16, hits)
	})
}
