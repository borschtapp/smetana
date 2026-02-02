package domain

import (
	"time"

	"github.com/google/uuid"
)

type UserToken struct {
	UserID  uuid.UUID `gorm:"type:char(36);index"`
	Type    string
	Token   string
	Expires time.Time
	Created time.Time `gorm:"autoCreateTime" json:"-"`

	User *User `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
}
