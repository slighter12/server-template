package usecase_test

import (
	"context"
	"errors"
	"testing"

	"server-template/config"
	"server-template/internal/domain/entity"
	"server-template/internal/domain/entity/user"
	"server-template/internal/infrastructure/auth"
	"server-template/internal/usecase"
)

type fakeRepo struct {
	users map[string]*entity.User // by email
	err   error                   // returned by FindByEmail when not ErrUserNotFound-able
}

func (r *fakeRepo) Create(_ context.Context, u *entity.User) error {
	r.users[u.Email] = u

	return nil
}

func (r *fakeRepo) FindByEmail(_ context.Context, email string) (*entity.User, error) {
	if r.err != nil {
		return nil, r.err
	}
	if u, ok := r.users[email]; ok {
		return u, nil
	}

	return nil, entity.ErrUserNotFound
}

func (r *fakeRepo) FindByID(_ context.Context, id string) (*entity.User, error) {
	for _, u := range r.users {
		if u.ID == id {
			return u, nil
		}
	}

	return nil, entity.ErrUserNotFound
}

func newUC() (*fakeRepo, func(context.Context, string, string) (*entity.User, error), func(context.Context, string, string) (*entity.User, error)) {
	cfg := &config.Config{}
	cfg.Auth.Argon2 = config.Argon2Config{Memory: 8, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 16, MaxConcurrent: 2}
	repo := &fakeRepo{users: map[string]*entity.User{}}
	uc := usecase.NewAuthUseCase(repo, auth.NewArgon2idHasher(cfg))

	return repo, uc.Register, uc.Login
}

func TestRegister(t *testing.T) {
	repo, register, _ := newUC()
	ctx := t.Context()

	for _, pw := range []string{"short1A", "alllowercase1", "ALLUPPERCASE1", "NoDigitsHere", string(make([]byte, 73))} {
		if _, err := register(ctx, "a@b.com", pw); !errors.Is(err, entity.ErrWeakPassword) {
			t.Errorf("password %q: err = %v, want ErrWeakPassword", pw, err)
		}
	}

	u, err := register(ctx, "a@b.com", "Passw0rdOK")
	if err != nil {
		t.Fatal(err)
	}
	if u.ID == "" || u.Status != user.UserStatusActive || u.Password == "Passw0rdOK" || repo.users["a@b.com"] != u {
		t.Fatalf("unexpected user %+v", u)
	}

	if _, err := register(ctx, "a@b.com", "Passw0rdOK"); !errors.Is(err, entity.ErrEmailTaken) {
		t.Fatalf("err = %v, want ErrEmailTaken", err)
	}

	repo.err = errors.New("db down")
	if _, err := register(ctx, "c@d.com", "Passw0rdOK"); err == nil || errors.Is(err, entity.ErrEmailTaken) {
		t.Fatalf("repo failure must pass through, got %v", err)
	}
}

func TestLogin(t *testing.T) {
	_, register, login := newUC()
	ctx := t.Context()

	if _, err := register(ctx, "a@b.com", "Passw0rdOK"); err != nil {
		t.Fatal(err)
	}

	u, err := login(ctx, "a@b.com", "Passw0rdOK")
	if err != nil || u.Email != "a@b.com" {
		t.Fatalf("login = %+v, %v", u, err)
	}

	_, errWrong := login(ctx, "a@b.com", "WrongPass1")
	_, errMissing := login(ctx, "nobody@b.com", "Passw0rdOK")
	if !errors.Is(errWrong, entity.ErrInvalidCredentials) || !errors.Is(errMissing, entity.ErrInvalidCredentials) {
		t.Fatalf("errWrong = %v, errMissing = %v", errWrong, errMissing)
	}
	if errWrong.Error() != errMissing.Error() {
		t.Fatalf("errors must be indistinguishable: %q vs %q", errWrong, errMissing)
	}
}
