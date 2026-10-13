package usecase

import (
	"context"
	"errors"
	"fmt"
	"uuid"

	"server-template/internal/domain/entity"
	"server-template/internal/domain/entity/user"
	"server-template/internal/domain/repository"
	"server-template/internal/domain/service"
	"server-template/internal/domain/usecase"
	"server-template/internal/pkg/apperr"
)

type authUseCase struct {
	userRepo repository.UserRepository
	hasher   service.PasswordHasher
}

func NewAuthUseCase(userRepo repository.UserRepository, hasher service.PasswordHasher) usecase.AuthUseCase {
	return &authUseCase{
		userRepo: userRepo,
		hasher:   hasher,
	}
}

func (uc *authUseCase) Register(ctx context.Context, email, password string) (*entity.User, error) {
	if err := entity.ValidatePassword(password); err != nil {
		return nil, err
	}

	// Repositories return entity.ErrUserNotFound when the email is free.
	_, err := uc.userRepo.FindByEmail(ctx, email)
	switch {
	case err == nil:
		return nil, entity.ErrEmailTaken
	case !errors.Is(err, entity.ErrUserNotFound):
		return nil, err
	}

	hash, err := uc.hasher.Hash(ctx, password)
	if err != nil {
		return nil, apperr.Wrap(fmt.Errorf("hash password: %w", err), apperr.ErrInternal)
	}

	u := &entity.User{
		ID:       uuid.NewV7().String(),
		Email:    email,
		Password: hash,
		Status:   user.UserStatusActive,
	}
	if err := u.Validate(); err != nil {
		return nil, err
	}

	// A concurrent registration surfaces as entity.ErrEmailTaken from the repository.
	if err := uc.userRepo.Create(ctx, u); err != nil {
		return nil, err
	}

	return u, nil
}

func (uc *authUseCase) Login(ctx context.Context, email, password string) (*entity.User, error) {
	u, err := uc.userRepo.FindByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, entity.ErrUserNotFound) {
			return nil, entity.ErrInvalidCredentials
		}

		return nil, err
	}

	ok, err := uc.hasher.Verify(ctx, password, u.Password)
	if err != nil {
		return nil, apperr.Wrap(fmt.Errorf("verify password: %w", err), apperr.ErrInternal)
	}
	if !ok {
		return nil, entity.ErrInvalidCredentials
	}

	return u, nil
}

func (uc *authUseCase) GetUserByID(ctx context.Context, userID string) (*entity.User, error) {
	return uc.userRepo.FindByID(ctx, userID)
}
