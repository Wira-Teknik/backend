package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PaymentDetail struct {
	ID              uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	PaymentID       uuid.UUID      `gorm:"type:uuid;not null;index" json:"payment_id"`
	InvoiceID       uuid.UUID      `gorm:"type:uuid;not null;index" json:"invoice_id"`
	AllocatedAmount float64        `gorm:"not null" json:"allocated_amount"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`
}
