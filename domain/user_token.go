package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const TokenTypeRefresh = "refresh"
const TokenTypeHouseholdInvite = "household_invite"
const TokenTypePasswordReset = "password_reset"

type UserToken struct {
	ID      uuid.UUID `gorm:"type:char(36);primaryKey"`
	UserID  uuid.UUID `gorm:"type:char(36);index" json:"user_id"`
	Type    string    `gorm:"index:idx_token_type"`
	Token   string    `gorm:"index:idx_token_type"`
	Expires time.Time
	Created time.Time `gorm:"autoCreateTime" json:"-"`

	User *User `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
}

func (t *UserToken) BeforeCreate(_ *gorm.DB) error {
	if t.ID == uuid.Nil {
		var err error
		t.ID, err = uuid.NewV7()
		return err
	}
	return nil
}
