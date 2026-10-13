package entity

import (
	"net/mail"
	"time"
	"unicode"

	"server-template/internal/domain/entity/user"
	"server-template/internal/pkg/apperr"
)

// MinPasswordLength and MaxPasswordLength bound a plaintext password in bytes.
// 72 matches the limit of common password hashes and the HTTP validate tags.
const (
	MinPasswordLength = 8
	MaxPasswordLength = 72
)

// User 代表使用者實體。
//
// The password column holds an encoded password hash (argon2id PHC string,
// roughly 100 characters), never plaintext; hashing is done by service.PasswordHasher.
type User struct {
	ID        string          `json:"id" gorm:"primaryKey;type:uuid;default:gen_random_uuid()"`
	Name      string          `json:"name" gorm:"type:varchar(32);not null;default:''"`
	Email     string          `json:"email" gorm:"type:varchar(255);uniqueIndex;not null"` // RFC 5322 標準規定 email 最大長度為 255 字元
	Password  string          `json:"-" gorm:"type:varchar(255);not null"`                 // encoded hash, 255 leaves room for argon2id PHC strings and parameter changes
	Status    user.UserStatus `json:"status" gorm:"type:integer;not null;default:1"`       // default must equal user.UserStatusActive
	CreatedAt time.Time       `json:"created_at" gorm:"type:timestamp with time zone;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt time.Time       `json:"updated_at" gorm:"type:timestamp with time zone;not null;default:CURRENT_TIMESTAMP;autoUpdateTime"`
}

// Validate checks the profile fields (not the password).
func (u *User) Validate() error {
	if _, err := mail.ParseAddress(u.Email); err != nil {
		return apperr.ErrInvalidInput.WithMessage("invalid email format")
	}

	if len(u.Name) > 32 {
		return apperr.ErrInvalidInput.WithMessage("name must be at most 32 characters")
	}

	return nil
}

// ValidatePassword is the single password-strength rule: 8 to 72 bytes with at
// least one upper case letter, one lower case letter and one digit. Violations
// return ErrWeakPassword.
func ValidatePassword(password string) error {
	if len(password) < MinPasswordLength || len(password) > MaxPasswordLength {
		return ErrWeakPassword
	}

	var upper, lower, digit bool
	for _, r := range password {
		upper = upper || unicode.IsUpper(r)
		lower = lower || unicode.IsLower(r)
		digit = digit || unicode.IsDigit(r)
	}
	if !upper || !lower || !digit {
		return ErrWeakPassword
	}

	return nil
}
