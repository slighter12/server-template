package user

type UserStatus int

const (
	UserStatusActive UserStatus = iota
	UserStatusInactive
)
