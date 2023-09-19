package domain

import (
	"github.com/borschtapp/krip"
)

type RecipeInstruction struct {
	ID       uint64  `gorm:"primaryKey" json:"id"`
	RecipeID uint64  `json:"-"`
	Order    uint8   `json:"order,omitempty"`
	Parent   *uint8  `json:"-"`
	Title    *string `json:"title,omitempty"`
	Text     string  `json:"text,omitempty"`
	Url      *string `json:"url,omitempty"`
	Image    *string `json:"image,omitempty"`
	Video    *string `json:"video,omitempty"`

	Recipe Recipe `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
}

func FromKripHowToStep(howToSection *krip.HowToStep) *RecipeInstruction {
	model := &RecipeInstruction{}
	if len(howToSection.Name) != 0 {
		model.Title = &howToSection.Name
	}
	if len(howToSection.Text) != 0 {
		model.Text = howToSection.Text
	}
	if len(howToSection.Url) != 0 {
		model.Url = &howToSection.Url
	}
	if len(howToSection.Image) != 0 {
		model.Image = &howToSection.Image
	}
	if len(howToSection.Video) != 0 {
		model.Video = &howToSection.Video
	}
	return model
}
