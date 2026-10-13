//go:build integration

package conn_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func mongoScratchCollection(t *testing.T) *mongo.Collection {
	t.Helper()

	coll := d.Mongo.Database("integration_test").Collection(uniq("it_mongo"))
	t.Cleanup(func() { _ = coll.Drop(cleanupCtx(t)) })

	return coll
}

// isReplicaSet reports whether the server can run multi-document transactions
// (docker compose runs a standalone mongod, which cannot).
func isReplicaSet(ctx context.Context, t *testing.T) bool {
	t.Helper()

	var hello bson.M
	require.NoError(t, d.Mongo.Database("admin").RunCommand(ctx, bson.D{{Key: "hello", Value: 1}}).Decode(&hello))
	_, ok := hello["setName"]

	return ok
}

func TestMongoQueries(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	var build bson.M
	require.NoError(t, d.Mongo.Database("admin").RunCommand(ctx, bson.D{{Key: "buildInfo", Value: 1}}).Decode(&build))
	t.Logf("mongo version=%v", build["version"])

	t.Run("insert one many and find", func(t *testing.T) {
		t.Parallel()

		coll := mongoScratchCollection(t)

		one, err := coll.InsertOne(ctx, bson.M{"name": "solo", "score": 1})
		require.NoError(t, err)
		assert.NotNil(t, one.InsertedID)

		many, err := coll.InsertMany(ctx, []any{
			bson.M{"name": "a", "score": 10, "team": "x"},
			bson.M{"name": "b", "score": 30, "team": "x"},
			bson.M{"name": "c", "score": 20, "team": "y"},
			bson.M{"name": "d", "score": 40, "team": "y"},
		})
		require.NoError(t, err)
		assert.Len(t, many.InsertedIDs, 4)

		var got bson.M
		require.NoError(t, coll.FindOne(ctx, bson.M{"name": "c"}).Decode(&got))
		assert.EqualValues(t, 20, got["score"])
		assert.ErrorIs(t, coll.FindOne(ctx, bson.M{"name": "missing"}).Err(), mongo.ErrNoDocuments)

		// Filter with a comparison operator, sort descending and limit.
		cur, err := coll.Find(ctx, bson.M{"score": bson.M{"$gte": 10}},
			options.Find().SetSort(bson.D{{Key: "score", Value: -1}}).SetLimit(2))
		require.NoError(t, err)
		var top []bson.M
		require.NoError(t, cur.All(ctx, &top))
		require.Len(t, top, 2)
		assert.Equal(t, "d", top[0]["name"])
		assert.Equal(t, "b", top[1]["name"])

		n, err := coll.CountDocuments(ctx, bson.M{"team": "x"})
		require.NoError(t, err)
		assert.EqualValues(t, 2, n)
	})

	t.Run("update set and inc", func(t *testing.T) {
		t.Parallel()

		coll := mongoScratchCollection(t)
		_, err := coll.InsertOne(ctx, bson.M{"name": "u", "score": 5})
		require.NoError(t, err)

		res, err := coll.UpdateOne(ctx, bson.M{"name": "u"},
			bson.M{"$set": bson.M{"note": "hello"}, "$inc": bson.M{"score": 3}})
		require.NoError(t, err)
		assert.EqualValues(t, 1, res.MatchedCount)
		assert.EqualValues(t, 1, res.ModifiedCount)

		var got bson.M
		require.NoError(t, coll.FindOne(ctx, bson.M{"name": "u"}).Decode(&got))
		assert.Equal(t, "hello", got["note"])
		assert.EqualValues(t, 8, got["score"])

		// Upsert creates the document when nothing matches.
		up, err := coll.UpdateOne(ctx, bson.M{"name": "fresh"}, bson.M{"$inc": bson.M{"score": 1}}, options.UpdateOne().SetUpsert(true))
		require.NoError(t, err)
		assert.EqualValues(t, 1, up.UpsertedCount)
	})

	t.Run("unique index and duplicate key error", func(t *testing.T) {
		t.Parallel()

		coll := mongoScratchCollection(t)
		name, err := coll.Indexes().CreateOne(ctx, mongo.IndexModel{
			Keys:    bson.D{{Key: "email", Value: 1}},
			Options: options.Index().SetUnique(true),
		})
		require.NoError(t, err)
		assert.Equal(t, "email_1", name)

		_, err = coll.InsertOne(ctx, bson.M{"email": "a@example.com"})
		require.NoError(t, err)

		_, err = coll.InsertOne(ctx, bson.M{"email": "a@example.com"})
		require.Error(t, err)
		assert.True(t, mongo.IsDuplicateKeyError(err), "want duplicate key error, got %v", err)

		_, err = coll.InsertMany(ctx, []any{bson.M{"email": "b@example.com"}, bson.M{"email": "a@example.com"}})
		require.Error(t, err)
		assert.True(t, mongo.IsDuplicateKeyError(err))
	})

	t.Run("aggregation match group", func(t *testing.T) {
		t.Parallel()

		coll := mongoScratchCollection(t)
		_, err := coll.InsertMany(ctx, []any{
			bson.M{"team": "x", "score": 10, "active": true},
			bson.M{"team": "x", "score": 30, "active": true},
			bson.M{"team": "x", "score": 99, "active": false},
			bson.M{"team": "y", "score": 20, "active": true},
		})
		require.NoError(t, err)

		// One accumulator per pipeline keeps the test runnable on FerretDB too
		// (v1.x mis-evaluates a second $sum in the same $group); real MongoDB
		// accepts any number of accumulators.
		group := func(acc bson.D) []bson.M {
			cur, err := coll.Aggregate(ctx, mongo.Pipeline{
				{{Key: "$match", Value: bson.M{"active": true}}},
				{{Key: "$group", Value: bson.D{{Key: "_id", Value: "$team"}, {Key: "v", Value: acc}}}},
				{{Key: "$sort", Value: bson.D{{Key: "_id", Value: 1}}}},
			})
			require.NoError(t, err)

			var out []bson.M
			require.NoError(t, cur.All(ctx, &out))

			return out
		}

		totals := group(bson.D{{Key: "$sum", Value: "$score"}})
		require.Len(t, totals, 2)
		assert.Equal(t, "x", totals[0]["_id"])
		assert.EqualValues(t, 40, totals[0]["v"], "inactive x row (99) is excluded by $match")
		assert.Equal(t, "y", totals[1]["_id"])
		assert.EqualValues(t, 20, totals[1]["v"])

		counts := group(bson.D{{Key: "$sum", Value: 1}})
		require.Len(t, counts, 2)
		assert.EqualValues(t, 2, counts[0]["v"])
		assert.EqualValues(t, 1, counts[1]["v"])
	})

	t.Run("delete many", func(t *testing.T) {
		t.Parallel()

		coll := mongoScratchCollection(t)
		_, err := coll.InsertMany(ctx, []any{bson.M{"k": 1}, bson.M{"k": 1}, bson.M{"k": 2}})
		require.NoError(t, err)

		res, err := coll.DeleteMany(ctx, bson.M{"k": 1})
		require.NoError(t, err)
		assert.EqualValues(t, 2, res.DeletedCount)

		n, err := coll.CountDocuments(ctx, bson.M{})
		require.NoError(t, err)
		assert.EqualValues(t, 1, n)
	})

	// Multi-document transactions need a replica set. docker-compose runs a
	// standalone mongod on purpose: a replica set would have to advertise a host
	// name that works both from the host and from the app container (which
	// shares the redis network namespace), so the transaction subtest is skipped
	// unless the target server happens to be a replica set.
	t.Run("transaction", func(t *testing.T) {
		t.Parallel()

		if !isReplicaSet(ctx, t) {
			t.Skip("mongo server is standalone (not a replica set): multi-document transactions are unsupported")
		}

		coll := mongoScratchCollection(t)
		// Collections must exist before a transaction touches them.
		require.NoError(t, d.Mongo.Database("integration_test").CreateCollection(ctx, coll.Name()))

		session, err := d.Mongo.StartSession()
		require.NoError(t, err)
		defer session.EndSession(ctx)

		_, err = session.WithTransaction(ctx, func(ctx context.Context) (any, error) {
			return coll.InsertOne(ctx, bson.M{"k": "commit"})
		})
		require.NoError(t, err)

		_, err = session.WithTransaction(ctx, func(ctx context.Context) (any, error) {
			if _, err := coll.InsertOne(ctx, bson.M{"k": "rollback"}); err != nil {
				return nil, err
			}

			return nil, context.Canceled // any error aborts the transaction
		})
		require.Error(t, err)

		n, err := coll.CountDocuments(ctx, bson.M{"k": "commit"})
		require.NoError(t, err)
		assert.EqualValues(t, 1, n)
		n, err = coll.CountDocuments(ctx, bson.M{"k": "rollback"})
		require.NoError(t, err)
		assert.EqualValues(t, 0, n)
	})
}
