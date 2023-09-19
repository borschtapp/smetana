package domain

import (
	"time"

	"gorm.io/gorm"

	"borscht.app/smetana/pkg/utils"
)

type UnitTag struct {
	UnitID uint
	Tag    string `gorm:"primaryKey"`
}

type Unit struct {
	ID      uint           `gorm:"primaryKey" json:"id"`
	Name    string         `json:"name"`
	Updated time.Time      `gorm:"autoUpdateTime" json:"updated"`
	Created time.Time      `gorm:"autoCreateTime" json:"created"`
	Deleted gorm.DeletedAt `gorm:"index" json:"-"`

	Tags []UnitTag `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"-"`
}

func (u *Unit) AfterCreate(tx *gorm.DB) (err error) {
	var tags = []UnitTag{
		{UnitID: u.ID, Tag: utils.CreateTag(u.Name)},
	}

	tx.Create(&tags)
	return
}
