package usecase

import (
	"context"

	"server-template/internal/domain/entity"
	"server-template/internal/domain/repository"
	"server-template/internal/domain/usecase"

	"github.com/google/uuid"
	"github.com/pkg/errors"
	"go.uber.org/fx"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type authUseCase struct {
	fx.In

	userRepo repository.UserRepository
}

func NewAuthUseCase(userRepo repository.UserRepository) usecase.AuthUseCase {
	return &authUseCase{
		userRepo: userRepo,
	}
}

func (uc *authUseCase) Register(ctx context.Context, email, hashedPassword string) (*entity.User, error) {
	// 檢查用戶是否已存在
	existingUser, err := uc.userRepo.FindByEmail(ctx, email)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errors.Wrap(err, "failed to check existing user")
	}
	if existingUser != nil {
		return nil, errors.New("user already exists")
	}

	// 創建新用戶
	user := &entity.User{
		ID:       uuid.New().String(),
		Email:    email,
		Password: hashedPassword,
	}

	// 驗證用戶資料
	if err := user.Validate(); err != nil {
		return nil, errors.Wrap(err, "invalid user data")
	}

	// 加密密碼
	if err := user.HashPassword(); err != nil {
		return nil, errors.Wrap(err, "vertify password failed")
	}

	if err := uc.userRepo.Create(ctx, user); err != nil {
		return nil, errors.Wrap(err, "failed to create user")
	}

	return user, nil
}

func (uc *authUseCase) Login(ctx context.Context, email, hashedPassword string) (*entity.User, error) {
	user, err := uc.userRepo.FindByEmail(ctx, email)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find user")
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(hashedPassword))
	if err != nil {
		return nil, errors.New("invalid credentials")
	}

	return user, nil
}

func (uc *authUseCase) GetUserByID(ctx context.Context, userID string) (*entity.User, error) {
	user, err := uc.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find user by ID")
	}

	return user, nil
}
