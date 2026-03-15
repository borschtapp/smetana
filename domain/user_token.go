package domain

import (
	"time"

	"github.com/google/uuid"
)

const TokenTypeRefresh = "refresh"
const TokenTypeHouseholdInvite = "household_invite"

type UserToken struct {
	ID      uuid.UUID `gorm:"type:char(36);primaryKey"`
	UserID  uuid.UUID `gorm:"type:char(36);index"`
	Type    string
	Token   string
	Expires time.Time
	Created time.Time `gorm:"autoCreateTime" json:"-"`

	User *User `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
}
