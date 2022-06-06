package model

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
