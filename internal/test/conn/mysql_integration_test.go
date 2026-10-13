//go:build integration

package conn_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// myThing is a scratch model covering the column types the app is likely to
// use; the table name is set per test with db.Table.
type myThing struct {
	ID        uint64         `gorm:"primaryKey;autoIncrement"` // bigint unsigned AUTO_INCREMENT
	Code      string         `gorm:"type:varchar(64);not null;uniqueIndex"`
	Note      string         `gorm:"type:text"`
	Meta      string         `gorm:"type:json"`
	Score     int            `gorm:"not null;default:0"`
	Active    bool           `gorm:"not null;default:false"`
	At        time.Time      `gorm:"type:datetime(6);not null"`
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

func mysqlScratchTable(t *testing.T) (*gorm.DB, string) {
	t.Helper()

	table := uniq("it_my")
	db := d.MySQL.WithContext(t.Context()).Table(table).Session(&gorm.Session{}) // Session: every chain call clones the statement
	require.NoError(t, db.AutoMigrate(&myThing{}))
	t.Cleanup(func() { d.MySQL.WithContext(cleanupCtx(t)).Exec(fmt.Sprintf("DROP TABLE IF EXISTS `%s`", table)) })

	return db, table
}

func TestMySQLGORMSyntax(t *testing.T) {
	t.Parallel()

	var version string
	require.NoError(t, d.MySQL.WithContext(t.Context()).Raw("SELECT VERSION()").Scan(&version).Error)
	t.Logf("mysql VERSION()=%s", version)
	require.NotEmpty(t, version)

	t.Run("AutoMigrate schema and create", func(t *testing.T) {
		t.Parallel()

		db, table := mysqlScratchTable(t)

		cols := map[string]string{}
		types, err := db.Migrator().ColumnTypes(&myThing{})
		require.NoError(t, err)
		for _, c := range types {
			cols[c.Name()], _ = c.ColumnType()
		}
		assert.Contains(t, cols["code"], "varchar(64)")
		assert.Contains(t, cols["id"], "bigint")
		assert.Contains(t, cols["id"], "unsigned")
		assert.Contains(t, cols["at"], "datetime(6)")
		assert.Contains(t, cols, "meta")
		assert.True(t, db.Migrator().HasTable(table))
		assert.True(t, db.Migrator().HasIndex(&myThing{}, "Code"))

		// Re-running AutoMigrate on an existing table is a no-op, not an error.
		require.NoError(t, db.AutoMigrate(&myThing{}))

		at := time.Now().Truncate(time.Microsecond)
		row := myThing{Code: "c1", Note: "héllo 世界 \U0001F680", Meta: `{"tags":["a","b"],"n":1}`, Score: 7, Active: true, At: at}
		require.NoError(t, db.Create(&row).Error)
		assert.NotZero(t, row.ID, "AUTO_INCREMENT id is filled in")

		var got myThing
		require.NoError(t, db.First(&got, row.ID).Error)
		assert.Equal(t, row.Note, got.Note)
		assert.JSONEq(t, row.Meta, got.Meta)
		assert.True(t, got.Active)
		assert.Equal(t, 7, got.Score)
		assert.True(t, at.Equal(got.At), "datetime(6) keeps microseconds: want %s got %s", at, got.At)
	})

	t.Run("batch insert query update", func(t *testing.T) {
		t.Parallel()

		db, _ := mysqlScratchTable(t)
		base := time.Now().Truncate(time.Microsecond)
		rows := make([]myThing, 0, 10)
		for i := range 10 {
			rows = append(rows, myThing{
				Code: fmt.Sprintf("b%02d", i), Score: i, Active: i%2 == 0, Meta: `{}`, At: base.Add(time.Duration(i) * time.Second),
			})
		}
		require.NoError(t, db.CreateInBatches(&rows, 4).Error)
		for _, r := range rows {
			assert.NotZero(t, r.ID)
		}

		var count int64
		require.NoError(t, db.Count(&count).Error)
		assert.EqualValues(t, 10, count)

		var page []myThing
		require.NoError(t, db.Where("active = ? AND score >= ?", true, 2).Order("score DESC").Limit(2).Offset(1).Find(&page).Error)
		require.Len(t, page, 2)
		assert.Equal(t, []int{6, 4}, []int{page[0].Score, page[1].Score})

		var first myThing
		require.NoError(t, db.Where("code = ?", "b03").First(&first).Error)
		assert.Equal(t, 3, first.Score)
		assert.ErrorIs(t, db.Where("code = ?", "nope").First(&myThing{}).Error, gorm.ErrRecordNotFound)

		// Updates with a struct skips zero values; the map form writes them.
		res := db.Model(&myThing{}).Where("code = ?", "b03").Updates(myThing{Note: "updated", Score: 100})
		require.NoError(t, res.Error)
		assert.EqualValues(t, 1, res.RowsAffected)
		require.NoError(t, db.Model(&myThing{}).Where("code = ?", "b03").Updates(map[string]any{"active": false}).Error)
		require.NoError(t, db.Model(&myThing{}).Where("code = ?", "b03").UpdateColumn("score", gorm.Expr("score + ?", 1)).Error)
		require.NoError(t, db.Where("code = ?", "b03").First(&first).Error)
		assert.Equal(t, "updated", first.Note)
		assert.Equal(t, 101, first.Score)
		assert.False(t, first.Active)
	})

	t.Run("upsert ON DUPLICATE KEY UPDATE", func(t *testing.T) {
		t.Parallel()

		db, _ := mysqlScratchTable(t)
		at := time.Now().Truncate(time.Microsecond)
		require.NoError(t, db.Create(&myThing{Meta: `{}`, Code: "u", Note: "first", Score: 1, At: at}).Error)

		upsert := clause.OnConflict{DoUpdates: clause.AssignmentColumns([]string{"note"})}
		require.NoError(t, db.Clauses(upsert).Create(&myThing{Meta: `{}`, Code: "u", Note: "second", Score: 99, At: at}).Error)

		var rows []myThing
		require.NoError(t, db.Find(&rows).Error)
		require.Len(t, rows, 1)
		assert.Equal(t, "second", rows[0].Note)
		assert.Equal(t, 1, rows[0].Score, "score was not in DoUpdates")

		// Expression form: score = score + VALUES(score) / new alias, built by gorm.
		counter := clause.OnConflict{DoUpdates: clause.Assignments(map[string]any{"score": gorm.Expr("score + ?", 5)})}
		require.NoError(t, db.Clauses(counter).Create(&myThing{Meta: `{}`, Code: "u", At: at}).Error)
		require.NoError(t, db.Find(&rows).Error)
		assert.Equal(t, 6, rows[0].Score)

		// DoNothing -> INSERT IGNORE-style no-op.
		require.NoError(t, db.Clauses(clause.OnConflict{DoNothing: true}).Create(&myThing{Meta: `{}`, Code: "u", Note: "ignored", At: at}).Error)
		require.NoError(t, db.Find(&rows).Error)
		assert.Equal(t, "second", rows[0].Note)

		// A plain duplicate insert is a real error.
		assert.Error(t, db.Create(&myThing{Meta: `{}`, Code: "u", At: at}).Error)
	})

	t.Run("soft delete", func(t *testing.T) {
		t.Parallel()

		db, _ := mysqlScratchTable(t)
		at := time.Now().Truncate(time.Microsecond)
		rows := []myThing{{Meta: `{}`, Code: "s1", At: at}, {Meta: `{}`, Code: "s2", At: at}}
		require.NoError(t, db.Create(&rows).Error)

		require.NoError(t, db.Delete(&myThing{}, rows[0].ID).Error)

		var visible, all int64
		require.NoError(t, db.Model(&myThing{}).Count(&visible).Error)
		require.NoError(t, db.Unscoped().Model(&myThing{}).Count(&all).Error)
		assert.EqualValues(t, 1, visible, "soft-deleted row is hidden")
		assert.EqualValues(t, 2, all)

		var gone myThing
		require.NoError(t, db.Unscoped().First(&gone, rows[0].ID).Error)
		assert.True(t, gone.DeletedAt.Valid, "deleted_at was set, row kept")
		assert.ErrorIs(t, db.First(&myThing{}, rows[0].ID).Error, gorm.ErrRecordNotFound)

		require.NoError(t, db.Unscoped().Delete(&myThing{}, rows[0].ID).Error)
		require.NoError(t, db.Unscoped().Model(&myThing{}).Count(&all).Error)
		assert.EqualValues(t, 1, all, "Unscoped delete removes the row")
	})

	t.Run("transaction commit and rollback", func(t *testing.T) {
		t.Parallel()

		db, _ := mysqlScratchTable(t)
		at := time.Now().Truncate(time.Microsecond)

		require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
			return tx.Create(&myThing{Meta: `{}`, Code: "tx-commit", At: at}).Error
		}))

		boom := errors.New("boom")
		err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Create(&myThing{Meta: `{}`, Code: "tx-rollback", At: at}).Error; err != nil {
				return err
			}

			return boom
		})
		require.ErrorIs(t, err, boom)

		// Explicit Begin/Rollback and a nested savepoint.
		tx := db.Begin()
		require.NoError(t, tx.Error)
		require.NoError(t, tx.Create(&myThing{Meta: `{}`, Code: "tx-manual", At: at}).Error)
		require.NoError(t, tx.SavePoint("sp1").Error)
		require.NoError(t, tx.Create(&myThing{Meta: `{}`, Code: "tx-after-sp", At: at}).Error)
		require.NoError(t, tx.RollbackTo("sp1").Error)
		require.NoError(t, tx.Commit().Error)

		var codes []string
		require.NoError(t, db.Order("code").Pluck("code", &codes).Error)
		assert.Equal(t, []string{"tx-commit", "tx-manual"}, codes)
	})
}
