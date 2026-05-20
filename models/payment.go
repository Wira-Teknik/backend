package models

import (
	"teknik/utils"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Payment struct {
	ID           uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	PaymentTotal float64        `gorm:"not null" json:"payment_total"`
	PaymentDate    utils.JSONDate     `gorm:"type:timestamp;not null" json:"payment_date"`
	Details        []PaymentDetail    `gorm:"foreignKey:PaymentID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"details"`
	CreatedAt    utils.JSONDateTime `json:"created_at"`
	UpdatedAt    utils.JSONDateTime `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}
