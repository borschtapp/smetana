package domain

import (
	"time"
)

type UserToken struct {
	UserID  uint
	Type    string
	Token   string
	Expires time.Time

	User User `json:"-"`
}
