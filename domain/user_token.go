package domain

import (
	"time"
)

type UserToken struct {
	UserID  uint
	User    User
	Type    string
	Token   string
	Expires time.Time
}
