package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Customer struct {
	ID              uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	CustomerName    string         `gorm:"type:varchar(255);not null" json:"customer_name"`
	CustomerEmail   string         `gorm:"type:varchar(255);uniqueIndex;not null" json:"customer_email"`
	CustomerPhone   string         `gorm:"type:varchar(50)" json:"customer_phone"`
	CustomerAddress string         `gorm:"type:text" json:"customer_address"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`
}
