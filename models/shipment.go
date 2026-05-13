package models

import (
	"teknik/utils"

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
	ShippingDate   utils.JSONDate `gorm:"type:timestamp;not null" json:"shipping_date"`
	ReceivedDate   *utils.JSONDate `gorm:"type:timestamp" json:"received_date"`
	ShippingStatus ShippingStatus `gorm:"type:varchar(20);not null;default:'dikirim';check:shipping_status IN ('dikirim', 'diterima')" json:"shipping_status"`
	Items          []ShipmentItem `gorm:"foreignKey:ShipmentID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"items"`
	CreatedAt      utils.JSONDateTime `json:"created_at"`
	UpdatedAt      utils.JSONDateTime `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
}
