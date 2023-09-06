package domain

import (
	"time"

	"gorm.io/gorm"
)

type Author struct {
	Name string
}

type Nutrition struct {
	Calories     uint
	Fat          uint
	SaturatedFat uint
	Carbohydrate uint
	Sugar        uint
	Fiber        uint
	Protein      uint
	Cholesterol  uint
	Sodium       uint
}

type Rating struct {
	Value uint
	Count uint
}

type RecipeIngredient struct {
	RecipeID     uint `gorm:"primaryKey"`
	IngredientID uint `gorm:"primaryKey"`
	Unit         string
	UnitModel    Unit `gorm:"foreignKey:Unit;references:Name"`
	Amount       uint
	Note         string
}

type Recipe struct {
	ID   uint `gorm:"primarykey" json:"id"`
	Name string
	//Image         []string
	Author      Author `gorm:"embedded;embeddedPrefix:author_"`
	Description string
	Category    string
	Cuisine     string
	//Keywords      []string
	DatePublished *time.Time
	Yield         uint8
	//Ingredient    []RecipeIngredient
	//Instructions  []Instruction
	PrepTime  uint16
	CookTime  uint16
	TotalTime uint16
	Nutrition Nutrition      `gorm:"embedded;embeddedPrefix:nutrition_"`
	Rating    Rating         `gorm:"embedded;embeddedPrefix:rating_"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deletedAt"`
	CreatedAt time.Time      `json:"createdAt"`
}
