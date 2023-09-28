package domain

import (
	"time"

	"gorm.io/gorm"

	"borscht.app/smetana/pkg/utils"
)

type FoodTag struct {
	FoodID uint
	Tag    string `gorm:"primaryKey"`
}

type Food struct {
	ID            uint           `gorm:"primaryKey" json:"id"`
	Name          string         `json:"name"`
	CategoryID    *string        `json:"category_id,omitempty"`
	Category      *FoodCategory  `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"  json:"category,omitempty"`
	Icon          *string        `json:"icon,omitempty"`
	DefaultUnitID *uint          `json:"default_unit_id,omitempty"`
	DefaultUnit   *Unit          `gorm:"foreignKey:DefaultUnitID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;" json:"default_unit,omitempty"`
	Updated       time.Time      `gorm:"autoUpdateTime" json:"-"`
	Created       time.Time      `gorm:"autoCreateTime" json:"-"`
	Deleted       gorm.DeletedAt `gorm:"index" json:"-"`

	Tags []*FoodTag `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
}

// TableName overrides the table name used by Food to `food`
func (f Food) TableName() string {
	return "food"
}

func (f *Food) AfterCreate(tx *gorm.DB) (err error) {
	var tags = []FoodTag{
		{FoodID: f.ID, Tag: utils.CreateTag(f.Name)},
	}

	tx.Create(&tags)
	return
}

type FoodCategory struct {
	ID       uint           `gorm:"primaryKey" json:"id"`
	ParentID *uint          `json:"-"`
	Parent   *FoodCategory  `gorm:"foreignKey:ParentID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;" json:"parent"`
	Name     string         `json:"name"`
	Updated  time.Time      `gorm:"autoUpdateTime" json:"-"`
	Created  time.Time      `gorm:"autoCreateTime" json:"-"`
	Deleted  gorm.DeletedAt `gorm:"index" json:"-"`
}
