package domain

import "github.com/google/uuid"

type RecipeNutrition struct {
	RecipeID    uuid.UUID `gorm:"primaryKey" json:"-"`
	ServingSize string    `json:"serving_size,omitempty" example:"1 plate"` // The serving size, in terms of the number of volume or mass.
	Calories    *float64  `json:"calories,omitempty" example:"450.5"`       // The number of calories.
	Fats        *float64  `json:"fat,omitempty" example:"15.2"`             // The number of grams of fat.
	FatSat      *float64  `json:"fat_saturated,omitempty" example:"5.1"`    // The number of grams of saturated fat.
	FatTrans    *float64  `json:"fat_trans,omitempty" example:"0.1"`        // The number of grams of trans fat.
	Cholesterol *float64  `json:"cholesterol,omitempty" example:"35.0"`     // The number of milligrams of cholesterol.
	Sodium      *float64  `json:"sodium,omitempty" example:"250.0"`         // The number of milligrams of sodium.
	Carbs       *float64  `json:"carbs,omitempty" example:"60.0"`           // The number of grams of carbohydrates.
	CarbSugar   *float64  `json:"carbs_sugar,omitempty" example:"10.0"`     // The number of grams of sugar.
	CarbFiber   *float64  `json:"carbs_fiber,omitempty" example:"4.5"`      // The number of grams of fiber.
	Protein     *float64  `json:"protein,omitempty" example:"22.0"`         // The number of grams of protein.

	// other minerals commonly found in recipes, not covered by schema.org

	Salt       *float64 `json:"salt,omitempty"`       // The number of grams of salt.
	Iron       *float64 `json:"iron,omitempty"`       // The number of milligrams of iron.
	Potassium  *float64 `json:"potassium,omitempty"`  // The number of milligrams of potassium.
	Calcium    *float64 `json:"calcium,omitempty"`    // The number of milligrams of calcium.
	Phosphorus *float64 `json:"phosphorus,omitempty"` // The number of milligrams of phosphorus.
	Magnesium  *float64 `json:"magnesium,omitempty"`  // The number of milligrams of magnesium.
	Zinc       *float64 `json:"zinc,omitempty"`       // The number of milligrams of zinc.
	Copper     *float64 `json:"copper,omitempty"`     // The number of milligrams of copper.
	Selenium   *float64 `json:"selenium,omitempty"`   // The number of micrograms of selenium.
	Manganese  *float64 `json:"manganese,omitempty"`  // The number of milligrams of manganese.
}
