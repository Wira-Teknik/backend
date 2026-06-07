package models

import (
	"teknik/utils"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type OrderStatus string

const (
	OrderStatusPending   OrderStatus = "pending"
	OrderStatusPartial   OrderStatus = "partial"
	OrderStatusShipped   OrderStatus = "shipped"
	OrderStatusCompleted OrderStatus = "completed"
)

type Order struct {
	ID               uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TransactionNo    string         `gorm:"type:varchar(255);uniqueIndex;not null" json:"transaction_no"`
	PoNo             string         `gorm:"type:varchar(255)" json:"po_no"`
	OrderDate        utils.JSONDate `gorm:"type:timestamp;not null" json:"order_date"`
	RecipientName    string         `gorm:"type:varchar(255);not null" json:"recipient_name"`
	RecipientAddress string         `gorm:"type:varchar(255)" json:"recipient_address"`
	RecipientPhone   string         `gorm:"type:varchar(50)" json:"recipient_phone"`
	RecipientEmail   string         `gorm:"type:varchar(255)" json:"recipient_email"`
	OrderStatus      OrderStatus    `gorm:"type:varchar(20);not null;default:'pending';check:order_status IN ('pending', 'partial', 'shipped', 'completed')" json:"order_status"`
	Items            []OrderItem    `gorm:"foreignKey:OrderID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"items"`
	Shipments          []Shipment     `gorm:"foreignKey:OrderID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"shipments"`
	Invoices           []Invoice      `gorm:"foreignKey:OrderID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"invoices"`
	TotalAmountToPay   float64        `gorm:"-" json:"total_amount_to_pay"`
	RemainingBalance   float64        `gorm:"-" json:"remaining_balance"`
	PaymentStatus      PaymentStatus  `gorm:"-" json:"payment_status"`
	Payments           []Payment      `gorm:"-" json:"payments"`
	CreatedAt        utils.JSONDateTime `json:"created_at"`
	UpdatedAt        utils.JSONDateTime `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`
}
