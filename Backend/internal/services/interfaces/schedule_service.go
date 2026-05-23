package interfaces

import (
	"Backend/internal/models"
	"time"
)

type ScheduleService interface {
	List(userID uint) ([]models.ScheduleEvent, error)
	ListByRange(userID uint, from, to time.Time) ([]models.ScheduleEvent, error)
	Create(userID uint, companyName, stage, title string, scheduledAt time.Time, notes string) (*models.ScheduleEvent, error)
	Get(userID, eventID uint) (*models.ScheduleEvent, error)
	Update(userID, eventID uint, companyName, stage, title string, scheduledAt time.Time, notes string) (*models.ScheduleEvent, error)
	Delete(userID, eventID uint) error
	ExportICS(userID uint) (string, error)
}
