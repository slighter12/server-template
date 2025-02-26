package entity

import (
	"time"

	"server-template/internal/domain/entity/user"

	"github.com/pkg/errors"
	"golang.org/x/crypto/bcrypt"
)

// User 代表使用者實體
type User struct {
	ID        string          `json:"id" gorm:"primaryKey"`
	Name      string          `json:"name" gorm:"column:name"`
	Email     string          `json:"email" gorm:"unique;not null;column:email"`
	Password  string          `json:"password" gorm:"column:password"`
	Status    user.UserStatus `json:"status" gorm:"column:status"` // 使用字串來表示 UserStatus
	CreatedAt time.Time       `json:"created_at" gorm:"column:created_at"`
	UpdatedAt time.Time       `json:"updated_at" gorm:"column:updated_at"`
}

func (u *User) Validate() error {
	if u.Email == "" {
		return errors.New("email is required")
	}

	return nil
}

func (u *User) HashPassword() error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(u.Password), bcrypt.DefaultCost)
	if err != nil {
		return errors.Wrap(err, "failed to hash password")
	}
	u.Password = string(hashedPassword)

	return nil
}
