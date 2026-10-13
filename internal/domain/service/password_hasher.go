// Package service defines interfaces for core, stateless domain logic.
package service

import "context"

// PasswordHasher abstracts password hashing (e.g. Argon2id) from the domain.
type PasswordHasher interface {
	// Hash generates a salted, encoded hash from a plaintext password.
	Hash(ctx context.Context, password string) (string, error)

	// Verify reports whether password matches the encoded hash.
	// A malformed encoded hash yields (false, nil); a canceled ctx yields an error.
	Verify(ctx context.Context, password, encoded string) (bool, error)
}
