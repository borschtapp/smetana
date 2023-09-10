package domain

import (
	"strconv"

	"github.com/borschtapp/krip"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type RecipeImage struct {
	ID          uuid.UUID `gorm:"type:char(36);primaryKey" json:"-"`
	Width       int       `json:"width,omitempty"`
	Height      int       `json:"height,omitempty"`
	Caption     string    `json:"caption,omitempty"`
	RemoteUrl   string    `json:"-"`
	DownloadUrl string    `json:"url"`

	RecipeID uint   `json:"-"`
	Recipe   Recipe `json:"-"`
}

func (r *RecipeImage) BeforeCreate(tx *gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	return nil
}

func (r *RecipeImage) FilePath() (string, string) {
	return "recipe", strconv.FormatInt(int64(r.RecipeID), 10) + "/" + r.ID.String() + ".jpg"
}

func FromKripImage(image *krip.ImageObject) *RecipeImage {
	model := &RecipeImage{}
	model.ID = uuid.New()
	model.RemoteUrl = image.Url
	model.Width = image.Width
	model.Height = image.Height
	model.Caption = image.Caption
	return model
}
