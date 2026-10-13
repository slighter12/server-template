// Package auth provides concrete implementations of authentication-related domain services.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"server-template/config"
	"server-template/internal/domain/service"

	"golang.org/x/crypto/argon2"
)

const (
	defaultMemory        uint32 = 64 * 1024
	defaultIterations    uint32 = 3
	defaultParallelism   uint8  = 1
	defaultSaltLength    uint32 = 16
	defaultKeyLength     uint32 = 32
	defaultMaxConcurrent        = 4
	argon2Version               = argon2.Version
)

var errInvalidHash = errors.New("invalid argon2id hash")

// Argon2idHasher implements service.PasswordHasher with Argon2id.
type Argon2idHasher struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
	saltLength  uint32
	keyLength   uint32
	sem         chan struct{}
}

var _ service.PasswordHasher = (*Argon2idHasher)(nil)

// NewArgon2idHasher builds a hasher from cfg.Auth.Argon2; zero values fall back to defaults.
func NewArgon2idHasher(cfg *config.Config) service.PasswordHasher {
	a := cfg.Auth.Argon2
	maxConc := a.MaxConcurrent
	if maxConc <= 0 {
		maxConc = defaultMaxConcurrent
	}

	return &Argon2idHasher{
		memory:      orDefault(a.Memory, defaultMemory),
		iterations:  orDefault(a.Iterations, defaultIterations),
		parallelism: orDefault(a.Parallelism, defaultParallelism),
		saltLength:  orDefault(a.SaltLength, defaultSaltLength),
		keyLength:   orDefault(a.KeyLength, defaultKeyLength),
		sem:         make(chan struct{}, maxConc),
	}
}

func orDefault[T comparable](v, def T) T {
	var zero T
	if v == zero {
		return def
	}

	return v
}

// Hash returns a PHC-style encoded Argon2id hash of password.
func (h *Argon2idHasher) Hash(ctx context.Context, password string) (string, error) {
	if err := h.acquire(ctx); err != nil {
		return "", err
	}
	defer h.release()

	salt := make([]byte, h.saltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}

	key := argon2.IDKey([]byte(password), salt, h.iterations, h.memory, h.parallelism, h.keyLength)

	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2Version, h.memory, h.iterations, h.parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

// Verify compares password with the encoded hash in constant time.
func (h *Argon2idHasher) Verify(ctx context.Context, password, encoded string) (bool, error) {
	p, err := parseHash(encoded)
	if err != nil {
		return false, nil //nolint:nilerr // malformed hash is simply a mismatch
	}

	if err := h.acquire(ctx); err != nil {
		return false, err
	}
	defer h.release()

	key := argon2.IDKey([]byte(password), p.salt, p.iterations, p.memory, p.parallelism, uint32(len(p.hash)))

	return subtle.ConstantTimeCompare(key, p.hash) == 1, nil
}

func (h *Argon2idHasher) acquire(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("acquire hasher slot: %w", err)
	}

	select {
	case <-ctx.Done():
		return fmt.Errorf("acquire hasher slot: %w", ctx.Err())
	case h.sem <- struct{}{}:
		return nil
	}
}

func (h *Argon2idHasher) release() { <-h.sem }

type parsedHash struct {
	memory, iterations uint32
	parallelism        uint8
	salt, hash         []byte
}

func parseHash(encoded string) (*parsedHash, error) {
	parts := strings.Split(strings.TrimPrefix(encoded, "$"), "$")
	if len(parts) != 5 || parts[0] != "argon2id" {
		return nil, errInvalidHash
	}

	v, ok := strings.CutPrefix(parts[1], "v=")
	if !ok {
		return nil, errInvalidHash
	}
	if ver, err := strconv.Atoi(v); err != nil || ver != argon2Version {
		return nil, errInvalidHash
	}

	var p parsedHash
	params := strings.Split(parts[2], ",")
	if len(params) != 3 {
		return nil, errInvalidHash
	}
	for i, prefix := range []string{"m=", "t=", "p="} {
		raw, ok := strings.CutPrefix(params[i], prefix)
		if !ok {
			return nil, errInvalidHash
		}
		bits := 32
		if i == 2 {
			bits = 8
		}
		n, err := strconv.ParseUint(raw, 10, bits)
		if err != nil || n == 0 {
			return nil, errInvalidHash
		}
		switch i {
		case 0:
			p.memory = uint32(n)
		case 1:
			p.iterations = uint32(n)
		default:
			p.parallelism = uint8(n)
		}
	}

	var err error
	if p.salt, err = base64.RawStdEncoding.DecodeString(parts[3]); err != nil || len(p.salt) == 0 {
		return nil, errInvalidHash
	}
	if p.hash, err = base64.RawStdEncoding.DecodeString(parts[4]); err != nil || len(p.hash) == 0 {
		return nil, errInvalidHash
	}

	return &p, nil
}
