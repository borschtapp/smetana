package domain

type RecipeIngredient struct {
	ID       uint64  `gorm:"primaryKey" json:"id"`
	RecipeID uint64  `json:"-"`
	Amount   float64 `json:"amount"`
	UnitID   *uint   `json:"-"`
	FoodID   *uint   `json:"-"`
	Note     *string `json:"note,omitempty"`
	Text     *string `json:"text,omitempty"` // original unparsed value

	Recipe Recipe `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
	Unit   *Unit  `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;" json:"unit,omitempty"`
	Food   *Food  `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;" json:"food,omitempty"`
}
