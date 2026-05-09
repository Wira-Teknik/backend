package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ShipmentItem struct {
	ID           uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	ShipmentID   uuid.UUID      `gorm:"type:uuid;not null;index" json:"shipment_id"`
	OrderItemID  uuid.UUID      `gorm:"type:uuid;not null;index" json:"order_item_id"`
	ShippingQty  int            `gorm:"not null" json:"shipping_qty"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}
