// Package conn holds shared helpers for the optional connection providers
// in its sub-packages (mysql, postgres, mongo, redis).
package conn

import (
	"maps"
	"slices"
)

// MainName is the preferred name of the default client.
const MainName = "main"

// DefaultName returns the name of the default client: "main" when present,
// otherwise the alphabetically first key. ok is false for an empty map.
func DefaultName[V any](clients map[string]V) (name string, ok bool) {
	if len(clients) == 0 {
		return "", false
	}
	if _, found := clients[MainName]; found {
		return MainName, true
	}

	return slices.Sorted(maps.Keys(clients))[0], true
}

// Default returns the default client (see DefaultName), or the zero value
// when clients is empty.
func Default[V any](clients map[string]V) V {
	name, ok := DefaultName(clients)
	if !ok {
		var zero V

		return zero
	}

	return clients[name]
}

// SortedNames returns the client names in a stable order.
func SortedNames[V any](clients map[string]V) []string {
	return slices.Sorted(maps.Keys(clients))
}
