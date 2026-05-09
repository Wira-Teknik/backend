package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PaymentStatus string

const (
	PaymentStatusUnpaid  PaymentStatus = "unpaid"
	PaymentStatusPartial PaymentStatus = "partial"
	PaymentStatusPaid    PaymentStatus = "paid"
)

type Invoice struct {
	ID               uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	ShipmentID       uuid.UUID      `gorm:"type:uuid;not null;index" json:"shipment_id"`
	InvoiceNo        string         `gorm:"type:varchar(255);uniqueIndex;not null" json:"invoice_no"`
	TotalAmount      float64        `gorm:"not null" json:"total_amount"`
	RemainingBalance float64        `gorm:"not null" json:"remaining_balance"`
	PaymentStatus    PaymentStatus  `gorm:"type:varchar(20);not null;default:'unpaid';check:payment_status IN ('unpaid', 'partial', 'paid')" json:"payment_status"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`
}
