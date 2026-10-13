//go:build integration

package conn_test

import (
	"strings"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// redisKeys returns n keys that share the hash tag {it} and therefore live in
// the same cluster slot (required for pipelines, MULTI/EXEC and Lua across keys).
// The part after the tag is unique per call. They are deleted on cleanup.
func redisKeys(t *testing.T, names ...string) []string {
	t.Helper()

	run := uniq("run")
	keys := make([]string, 0, len(names))
	for _, n := range names {
		keys = append(keys, "{it}:"+run+":"+n)
	}
	t.Cleanup(func() { _ = d.Redis.Del(cleanupCtx(t), keys...).Err() })

	return keys
}

func TestRedisCluster(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	info, err := d.Redis.Info(ctx, "server").Result()
	require.NoError(t, err)
	for line := range strings.Lines(info) {
		if strings.HasPrefix(line, "redis_version:") {
			t.Logf("redis %s", strings.TrimSpace(line))
		}
	}

	t.Run("cluster state", func(t *testing.T) {
		t.Parallel()

		out, err := d.Redis.ClusterInfo(ctx).Result()
		require.NoError(t, err)
		assert.Contains(t, out, "cluster_state:ok")
		assert.Contains(t, out, "cluster_slots_assigned:16384")
	})

	t.Run("set get ttl incr", func(t *testing.T) {
		t.Parallel()

		keys := redisKeys(t, "str", "counter", "nottl")

		require.NoError(t, d.Redis.Set(ctx, keys[0], "v", time.Minute).Err())
		got, err := d.Redis.Get(ctx, keys[0]).Result()
		require.NoError(t, err)
		assert.Equal(t, "v", got)

		ttl, err := d.Redis.TTL(ctx, keys[0]).Result()
		require.NoError(t, err)
		assert.Greater(t, ttl, 50*time.Second)
		assert.LessOrEqual(t, ttl, time.Minute)

		require.NoError(t, d.Redis.Set(ctx, keys[2], "v", 0).Err())
		ttl, err = d.Redis.TTL(ctx, keys[2]).Result()
		require.NoError(t, err)
		assert.EqualValues(t, -1, ttl, "no expiry")

		// Short TTL really expires.
		require.NoError(t, d.Redis.Set(ctx, keys[2], "v", 100*time.Millisecond).Err())
		assert.Eventually(t, func() bool {
			return d.Redis.Exists(ctx, keys[2]).Val() == 0
		}, 5*time.Second, 50*time.Millisecond)

		_, err = d.Redis.Get(ctx, keys[2]).Result()
		assert.ErrorIs(t, err, goredis.Nil)

		for want := int64(1); want <= 3; want++ {
			n, err := d.Redis.Incr(ctx, keys[1]).Result()
			require.NoError(t, err)
			assert.Equal(t, want, n)
		}
		n, err := d.Redis.IncrBy(ctx, keys[1], 10).Result()
		require.NoError(t, err)
		assert.EqualValues(t, 13, n)
	})

	t.Run("hash", func(t *testing.T) {
		t.Parallel()

		key := redisKeys(t, "hash")[0]
		added, err := d.Redis.HSet(ctx, key, map[string]any{"name": "alice", "age": 30}).Result()
		require.NoError(t, err)
		assert.EqualValues(t, 2, added)

		all, err := d.Redis.HGetAll(ctx, key).Result()
		require.NoError(t, err)
		assert.Equal(t, map[string]string{"name": "alice", "age": "30"}, all)

		n, err := d.Redis.HIncrBy(ctx, key, "age", 1).Result()
		require.NoError(t, err)
		assert.EqualValues(t, 31, n)
	})

	t.Run("pipeline on same slot", func(t *testing.T) {
		t.Parallel()

		keys := redisKeys(t, "p1", "p2", "p3")

		pipe := d.Redis.Pipeline()
		pipe.Set(ctx, keys[0], "a", time.Minute)
		pipe.Set(ctx, keys[1], "b", time.Minute)
		incr := pipe.Incr(ctx, keys[2])
		get := pipe.Get(ctx, keys[0])
		cmds, err := pipe.Exec(ctx)
		require.NoError(t, err)
		assert.Len(t, cmds, 4)
		assert.EqualValues(t, 1, incr.Val())
		assert.Equal(t, "a", get.Val())

		// Keys without a common hash tag are split per node/slot by the cluster
		// client and still work, but only same-slot keys are atomic.
		slotA, err := d.Redis.ClusterKeySlot(ctx, keys[0]).Result()
		require.NoError(t, err)
		slotB, err := d.Redis.ClusterKeySlot(ctx, keys[1]).Result()
		require.NoError(t, err)
		assert.Equal(t, slotA, slotB, "hash tag {it} pins the keys to one slot")
	})

	t.Run("MULTI EXEC", func(t *testing.T) {
		t.Parallel()

		keys := redisKeys(t, "tx1", "tx2")

		var incr *goredis.IntCmd
		cmds, err := d.Redis.TxPipelined(ctx, func(pipe goredis.Pipeliner) error {
			pipe.Set(ctx, keys[0], "x", time.Minute)
			incr = pipe.Incr(ctx, keys[1])
			pipe.Expire(ctx, keys[1], time.Minute)

			return nil
		})
		require.NoError(t, err)
		assert.Len(t, cmds, 3)
		assert.EqualValues(t, 1, incr.Val())

		got, err := d.Redis.Get(ctx, keys[0]).Result()
		require.NoError(t, err)
		assert.Equal(t, "x", got)

		// WATCH-based optimistic transaction.
		err = d.Redis.Watch(ctx, func(tx *goredis.Tx) error {
			n, err := tx.Get(ctx, keys[1]).Int64()
			if err != nil {
				return err
			}
			_, err = tx.TxPipelined(ctx, func(pipe goredis.Pipeliner) error {
				pipe.Set(ctx, keys[1], n+41, time.Minute)

				return nil
			})

			return err
		}, keys[1])
		require.NoError(t, err)
		final, err := d.Redis.Get(ctx, keys[1]).Int64()
		require.NoError(t, err)
		assert.EqualValues(t, 42, final)
	})

	t.Run("Lua EVAL", func(t *testing.T) {
		t.Parallel()

		keys := redisKeys(t, "lua1", "lua2")
		require.NoError(t, d.Redis.Set(ctx, keys[0], 5, time.Minute).Err())

		script := goredis.NewScript(`
			local a = tonumber(redis.call('GET', KEYS[1]))
			redis.call('SET', KEYS[2], a + tonumber(ARGV[1]))
			return redis.call('GET', KEYS[2])`)

		// Run uses EVALSHA and falls back to EVAL on NOSCRIPT.
		got, err := script.Run(ctx, d.Redis, keys, 7).Text()
		require.NoError(t, err)
		assert.Equal(t, "12", got)

		got, err = d.Redis.Eval(ctx, `return redis.call('GET', KEYS[2])`, keys).Text()
		require.NoError(t, err)
		assert.Equal(t, "12", got)
	})
}
