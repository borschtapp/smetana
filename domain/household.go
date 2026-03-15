package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Household struct {
	ID      uuid.UUID `gorm:"type:char(36);primaryKey" json:"id"`
	OwnerID uuid.UUID `gorm:"type:char(36)" json:"owner_id"`
	Name    string    `json:"name"`
	Updated time.Time `gorm:"autoUpdateTime" json:"-"`
	Created time.Time `gorm:"autoCreateTime" json:"-"`

	Members       []*User         `gorm:"foreignKey:HouseholdID" json:"members,omitempty"`
	Feeds         []*Feed         `gorm:"many2many:feed_subscriptions;" json:"feeds,omitempty"`
	Collections   []*Collection   `gorm:"foreignKey:HouseholdID" json:"collections,omitempty"`
	ShoppingLists []*ShoppingList `gorm:"foreignKey:HouseholdID" json:"shopping_lists,omitempty"`
}

func (h *Household) BeforeCreate(_ *gorm.DB) error {
	if h.ID == uuid.Nil {
		var err error
		h.ID, err = uuid.NewV7()
		return err
	}
	return nil
}

type HouseholdRepository interface {
	ByID(id uuid.UUID) (*Household, error)
	Create(household *Household) error
	Update(household *Household) error
	Delete(id uuid.UUID) error

	Members(householdID uuid.UUID, offset, limit int) ([]User, int64, error)
	FirstOtherMember(householdID, excludeUserID uuid.UUID) (*User, error)
}

type HouseholdService interface {
	ByID(id uuid.UUID, requesterHouseholdID uuid.UUID) (*Household, error)
	Update(household *Household, requesterHouseholdID uuid.UUID) error

	Members(householdID uuid.UUID, requesterHouseholdID uuid.UUID, offset, limit int) ([]User, int64, error)
	RemoveMember(householdID uuid.UUID, requesterID, requesterHouseholdID, targetUserID uuid.UUID) error

	ListInvites(householdID uuid.UUID, requesterID, requesterHouseholdID uuid.UUID) ([]UserToken, error)
	CreateInvite(householdID uuid.UUID, requesterID, requesterHouseholdID uuid.UUID) (*UserToken, error)
	RevokeInvite(householdID uuid.UUID, requesterHouseholdID uuid.UUID, code string) error
	JoinByInvite(joiningUserID uuid.UUID, code string) error
}
