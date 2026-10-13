package user

// UserStatus is stored in users.status. The values must stay consistent with
// the column default (1) in the migrations and the gorm tag on entity.User.
type UserStatus int

const (
	UserStatusInactive UserStatus = 0
	UserStatusActive   UserStatus = 1 // column default
)
