package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type RecipeIngredient struct {
	ID        uuid.UUID  `gorm:"type:char(36);primaryKey" json:"id"`
	RecipeID  uuid.UUID  `gorm:"type:char(36);index" json:"-"`
	Amount    *float64   `json:"amount,omitempty"`    // nil when unquantified (e.g. "to taste", "a pinch of")
	MaxAmount *float64   `json:"maxAmount,omitempty"` // upper bound for range quantities (e.g. "1–2 cups")
	UnitID    *uuid.UUID `gorm:"type:char(36);index" json:"unit_id,omitempty"`
	FoodID    *uuid.UUID `gorm:"type:char(36);index" json:"food_id,omitempty"`
	// Name is the ingredient name as written in this recipe (e.g. "carrots", "all-purpose flour").
	// May differ from Food.Name, which holds the deduplicated canonical form (e.g. "carrot").
	Name string `json:"name,omitempty"`
	// Description holds preparation notes extracted from the ingredient string (e.g. "finely diced", "at room temperature").
	Description *string `json:"description,omitempty"`
	// Category groups ingredients into named sections within a recipe (e.g. "For the sauce", "For the dough").
	Category *string `json:"category,omitempty"`
	// RawText is the original unparsed ingredient string from the source (e.g. "2 large carrots, diced").
	RawText string    `json:"raw_text,omitempty"`
	Updated time.Time `gorm:"autoUpdateTime" json:"-"`
	Created time.Time `gorm:"autoCreateTime" json:"-"`

	Recipe *Recipe `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
	Unit   *Unit   `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;" json:"unit,omitempty"`
	Food   *Food   `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;" json:"food,omitempty"`
}

func (ri *RecipeIngredient) BeforeCreate(_ *gorm.DB) error {
	if ri.ID == uuid.Nil {
		var err error
		ri.ID, err = uuid.NewV7()
		return err
	}
	return nil
}
