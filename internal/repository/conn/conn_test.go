package conn

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultName(t *testing.T) {
	_, ok := DefaultName(map[string]int{})
	assert.False(t, ok)

	name, ok := DefaultName(map[string]int{"zeta": 1, "main": 2, "alpha": 3})
	assert.True(t, ok)
	assert.Equal(t, "main", name)

	// Deterministic without "main": repeat to defeat map iteration order.
	for range 50 {
		name, ok = DefaultName(map[string]int{"zeta": 1, "beta": 2, "alpha": 3})
		assert.True(t, ok)
		assert.Equal(t, "alpha", name)
	}
}

func TestDefault(t *testing.T) {
	assert.Equal(t, 0, Default(map[string]int{}))
	assert.Equal(t, 3, Default(map[string]int{"b": 1, "a": 3}))
	assert.Nil(t, Default(map[string]*int(nil)))
}
