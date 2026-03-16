package domain

import (
	"time"

	"borscht.app/smetana/internal/storage"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type User struct {
	ID            uuid.UUID     `gorm:"type:char(36);primaryKey" json:"id"`
	HouseholdID   uuid.UUID     `gorm:"type:char(36);index" json:"-"`
	Name          string        `json:"name"`
	Email         string        `gorm:"uniqueIndex;not null" json:"email"`
	EmailVerified bool          `gorm:"default:false" json:"-"`
	Password      string        `json:"-"`
	ImagePath     *storage.Path `json:"image_url,omitempty"`
	Updated       time.Time     `gorm:"autoUpdateTime" json:"-"`
	Created       time.Time     `gorm:"autoCreateTime" json:"-"`

	Household *Household   `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
	Tokens    []*UserToken `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
	Recipes   []*Recipe    `gorm:"many2many:recipes_saved;" json:"recipes,omitempty"`
	Images    []*Image     `gorm:"polymorphic:Entity;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"images,omitempty"`
}

func (u *User) BeforeCreate(_ *gorm.DB) error {
	if u.ID == uuid.Nil {
		var err error
		u.ID, err = uuid.NewV7()
		return err
	}
	return nil
}

type UserRepository interface {
	ByID(id uuid.UUID) (*User, error)
	ByEmail(email string) (*User, error)
	ByEmailWithHousehold(email string) (*User, error)
	Create(user *User) error
	Update(user *User) error
	Delete(id uuid.UUID) error

	FindToken(tokenStr string, tokenType string) (*UserToken, error)
	FindTokensByUser(userID uuid.UUID, tokenType string) ([]UserToken, error)
	CreateToken(token *UserToken) error
	DeleteToken(tokenStr string) (bool, error)
}

type UserService interface {
	ByID(id uuid.UUID, requesterID uuid.UUID) (*User, error)
	Update(id uuid.UUID, requesterID uuid.UUID, name, email, currentPassword, newPassword *string) (*User, error)
	Delete(id uuid.UUID, requesterID uuid.UUID) error
}
