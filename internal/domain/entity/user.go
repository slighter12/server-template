package entity

import (
	"time"

	"server-template/internal/domain/entity/user"

	"github.com/pkg/errors"
	"golang.org/x/crypto/bcrypt"
)

// User 代表使用者實體
type User struct {
	ID        string          `json:"id" gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	Name      string          `json:"name" gorm:"type:varchar(32);not null"`
	Email     string          `json:"email" gorm:"type:varchar(255);uniqueIndex;not null"`
	Password  string          `json:"password" gorm:"type:varchar(60);not null"` // Password 欄位使用 varchar(60) 是因為 bcrypt 使用 DefaultCost (cost=10) 時，加密後的密碼長度固定為 60 字元
	Status    user.UserStatus `json:"status" gorm:"type:integer;not null;default:1"`
	CreatedAt time.Time       `json:"created_at" gorm:"type:timestamp with time zone;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt time.Time       `json:"updated_at" gorm:"type:timestamp with time zone;not null;default:CURRENT_TIMESTAMP;autoUpdateTime"`
}

func (u *User) Validate() error {
	if u.Email == "" {
		return errors.New("email is required")
	}

	if len(u.Name) > 32 {
		return errors.New("name must be less than 32 characters")
	}

	return u.validatePassword()
}

func (u *User) validatePassword() error {
	if len(u.Password) < 8 {
		return errors.New("password must be at least 8 characters long")
	}
	if len(u.Password) > 128 {
		return errors.New("password must be less than 128 characters")
	}

	return u.validatePasswordComplexity()
}

func (u *User) validatePasswordComplexity() error {
	var (
		hasUpper   bool
		hasLower   bool
		hasNumber  bool
		hasSpecial bool
	)

	for _, char := range u.Password {
		hasUpper = hasUpper || isUpperCase(char)
		hasLower = hasLower || isLowerCase(char)
		hasNumber = hasNumber || isNumber(char)
		hasSpecial = hasSpecial || isSpecialChar(char)
	}

	if !hasUpper || !hasLower || !hasNumber || !hasSpecial {
		return errors.New("password must contain at least one uppercase letter, one lowercase letter, one number, and one special character")
	}

	return nil
}

func isUpperCase(char rune) bool {
	return char >= 'A' && char <= 'Z'
}

func isLowerCase(char rune) bool {
	return char >= 'a' && char <= 'z'
}

func isNumber(char rune) bool {
	return char >= '0' && char <= '9'
}

// isSpecialChar 檢查是否為特殊符號，包含以下範圍：
// - ! " # $ % & ' ( ) * + , - . /  (ASCII 33-47)
// - : ; < = > ? @                  (ASCII 58-64)
// - [ \ ] ^ _ `                    (ASCII 91-96)
// - { | } ~                        (ASCII 123-126)
func isSpecialChar(char rune) bool {
	return (char >= '!' && char <= '/') ||
		(char >= ':' && char <= '@') ||
		(char >= '[' && char <= '`') ||
		(char >= '{' && char <= '~')
}

func (u *User) HashPassword() error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(u.Password), bcrypt.DefaultCost)
	if err != nil {
		return errors.Wrap(err, "failed to hash password")
	}
	u.Password = string(hashedPassword)

	return nil
}
