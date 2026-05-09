package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ShippingStatus string

const (
	ShippingStatusDikirim  ShippingStatus = "dikirim"
	ShippingStatusDiterima ShippingStatus = "diterima"
)

type Shipment struct {
	ID             uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	OrderID        uuid.UUID      `gorm:"type:uuid;not null;index" json:"order_id"`
	ShippingDate   time.Time      `gorm:"type:timestamp;not null" json:"shipping_date"`
	ReceivedDate   *time.Time     `gorm:"type:timestamp" json:"received_date"`
	ShippingStatus ShippingStatus `gorm:"type:varchar(20);not null;default:'dikirim';check:shipping_status IN ('dikirim', 'diterima')" json:"shipping_status"`
	Items          []ShipmentItem `gorm:"foreignKey:ShipmentID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"items"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
}
