package domain

import (
	"time"
)

type Unit struct {
	ID   uint16 `gorm:"primarykey" json:"id"`
	Name string
}

type Ingredient struct {
	ID        uint `gorm:"primarykey" json:"id"`
	Name      string
	UpdatedAt time.Time
	CreatedAt time.Time
}

type RecipeIngredient struct {
	RecipeID     uint `gorm:"primaryKey"`
	IngredientID uint `gorm:"primaryKey"`
	Unit         string
	UnitModel    Unit `gorm:"foreignKey:Unit;references:Name"`
	Amount       uint
	Note         string
}
