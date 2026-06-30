package models

import (
	"teknik/utils"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ShipmentItem struct {
	ID           uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	ShipmentID   uuid.UUID      `gorm:"type:uuid;not null;index" json:"shipment_id"`
	OrderItemID  uuid.UUID      `gorm:"type:uuid;not null;index" json:"order_item_id"`
	OrderItem    OrderItem      `gorm:"foreignKey:OrderItemID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;" json:"order_item"`
	ShippingQty  int            `gorm:"not null" json:"shipping_qty"`
	CreatedAt    utils.JSONDateTime `json:"created_at"`
	UpdatedAt    utils.JSONDateTime `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}
