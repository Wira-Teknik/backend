package models

import (
	"teknik/utils"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// OrderItem merepresentasikan detail barang/item dari pesanan (order).
type OrderItem struct {
	ID           uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	OrderID      uuid.UUID      `gorm:"type:uuid;not null;index" json:"order_id"`
	ProductName  string         `gorm:"type:varchar(255);not null" json:"product_name"`
	OrderQty     int            `gorm:"not null" json:"order_qty"`
	RemainingQty int            `gorm:"not null" json:"remaining_qty"`
	UnitPrice    float64        `gorm:"not null" json:"unit_price"`
	PPN          float64        `gorm:"not null" json:"ppn"`
	Subtotal     float64        `gorm:"not null" json:"subtotal"`
	CreatedAt    utils.JSONDateTime `json:"-"`
	UpdatedAt    utils.JSONDateTime `json:"-"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}
