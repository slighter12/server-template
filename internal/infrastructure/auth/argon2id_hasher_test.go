package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"testing/synctest"

	"server-template/config"
)

func testHasher(maxConc int) *Argon2idHasher {
	var cfg config.Config
	cfg.Auth.Argon2 = config.Argon2Config{Memory: 8, Iterations: 1, Parallelism: 1, MaxConcurrent: maxConc}

	return NewArgon2idHasher(&cfg).(*Argon2idHasher)
}

func TestHashVerifyRoundtrip(t *testing.T) {
	h := testHasher(2)
	enc, err := h.Hash(t.Context(), "s3cret!")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(enc, "$argon2id$v=19$m=8,t=1,p=1$") {
		t.Fatalf("unexpected encoding %q", enc)
	}
	ok, err := h.Verify(t.Context(), "s3cret!", enc)
	if err != nil || !ok {
		t.Fatalf("verify = %v, %v", ok, err)
	}
	enc2, _ := h.Hash(t.Context(), "s3cret!")
	if enc == enc2 {
		t.Fatal("salts must differ")
	}
}

func TestVerifyWrongPassword(t *testing.T) {
	h := testHasher(1)
	enc, _ := h.Hash(t.Context(), "right")
	ok, err := h.Verify(t.Context(), "wrong", enc)
	if err != nil || ok {
		t.Fatalf("verify = %v, %v", ok, err)
	}
}

func TestVerifyMalformed(t *testing.T) {
	h := testHasher(1)
	for _, enc := range []string{
		"", "plain", "$argon2i$v=19$m=8,t=1,p=1$YQ$YQ", "$argon2id$v=18$m=8,t=1,p=1$YQ$YQ",
		"$argon2id$v=19$m=8,t=1$YQ$YQ", "$argon2id$v=19$m=x,t=1,p=1$YQ$YQ",
		"$argon2id$v=19$m=8,t=1,p=1$!!$YQ", "$argon2id$v=19$m=8,t=1,p=1$YQ$!!",
	} {
		ok, err := h.Verify(t.Context(), "pw", enc)
		if ok || err != nil {
			t.Errorf("%q: got %v, %v", enc, ok, err)
		}
	}
}

func TestDefaults(t *testing.T) {
	h := NewArgon2idHasher(&config.Config{}).(*Argon2idHasher)
	if h.memory != defaultMemory || h.iterations != defaultIterations || cap(h.sem) != defaultMaxConcurrent {
		t.Fatalf("defaults not applied: %+v", h)
	}
}

func TestCanceledContext(t *testing.T) {
	h := testHasher(1)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := h.Hash(ctx, "pw"); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
}

func TestCancelWhileWaitingForSlot(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := testHasher(1)
		h.sem <- struct{}{} // occupy the only slot

		ctx, cancel := context.WithCancel(t.Context())
		errc := make(chan error, 1)
		go func() {
			_, err := h.Hash(ctx, "pw")
			errc <- err
		}()
		synctest.Wait() // goroutine is durably blocked on the semaphore
		cancel()
		synctest.Wait()
		if err := <-errc; !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v", err)
		}
	})
}

func BenchmarkHash(b *testing.B) {
	h := testHasher(4)
	for b.Loop() {
		if _, err := h.Hash(b.Context(), "benchmark-password"); err != nil {
			b.Fatal(err)
		}
	}
}
