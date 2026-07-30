package models

import (
	"teknik/utils"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Customer merepresentasikan model data pelanggan.
type Customer struct {
	ID              uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	CustomerName    string         `gorm:"type:varchar(255);not null" json:"customer_name"`
	CustomerEmail   string         `gorm:"type:varchar(255);uniqueIndex;not null" json:"customer_email"`
	CustomerPhone   string         `gorm:"type:varchar(50)" json:"customer_phone"`
	CustomerAddress string         `gorm:"type:text" json:"customer_address"`
	CreatedAt       utils.JSONDateTime `json:"created_at"`
	UpdatedAt       utils.JSONDateTime `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`
}
