package server

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseSize(t *testing.T) {
	tests := map[string]int64{"4M": 4 << 20, "512k": 512 << 10, "1GB": 1 << 30, "100": 100, " 2 K ": 2 << 10}
	for in, want := range tests {
		got, err := parseSize(in)
		assert.NoError(t, err, in)
		assert.Equal(t, want, got, in)
	}
	for _, in := range []string{"", "M", "-1K", "abc", "0", "9999999999P"} {
		_, err := parseSize(in)
		assert.Error(t, err, in)
	}
}
