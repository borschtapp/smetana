package domain

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

// HowToStep a step in the instructions https://schema.org/HowToStep
type HowToStep struct {
	Name  string `json:"name,omitempty"`
	Text  string `json:"text,omitempty"`
	Url   string `json:"url,omitempty"`
	Image string `json:"image,omitempty"`
	Video string `json:"video,omitempty"`
}

// HowToSection a group of steps in the instructions https://schema.org/HowToSection
type HowToSection struct {
	HowToStep              // because it's optional to have a group, we have to embed `HowToStep` here
	Steps     []*HowToStep `json:"itemListElement,omitempty"`
}
