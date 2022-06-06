package model

import (
	"time"

	"gorm.io/gorm"
)

type Book struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	Title     string         `json:"title"`
	Author    string         `json:"author"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deletedAt"`
	CreatedAt time.Time      `json:"createdAt"`
}
