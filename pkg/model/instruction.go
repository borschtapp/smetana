package model

import (
	"time"
)

type Instruction struct {
	RecipeID  uint  `gorm:"primarykey" json:"recipe_id"`
	Order     uint8 `gorm:"primarykey" json:"order"`
	Name      string
	Text      string
	Url       string
	Image     string
	UpdatedAt time.Time
	CreatedAt time.Time
}
