package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	JobStatusRunning = "running"
	JobStatusSuccess = "success"
	JobStatusError   = "error"
)

type SchedulerLog struct {
	ID           uuid.UUID  `gorm:"type:char(36);primaryKey" json:"id"`
	JobType      string     `gorm:"index" json:"job_type"`
	EntityID     *uuid.UUID `gorm:"type:char(36);index" json:"entity_id,omitempty"`
	StartedAt    time.Time  `json:"started_at"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
	Status       string     `json:"status"` // "running", "success", "error"
	ErrorMessage string     `json:"error_message,omitempty"`
	Metadata     string     `gorm:"type:text" json:"metadata,omitempty"` // stored as JSON string
}

func (l *SchedulerLog) BeforeCreate(_ *gorm.DB) error {
	if l.ID == uuid.Nil {
		var err error
		l.ID, err = uuid.NewV7()
		if err != nil {
			return err
		}
	}
	return nil
}

type SchedulerRepository interface {
	CreateLog(log *SchedulerLog) error
	UpdateLog(log *SchedulerLog) error
}
