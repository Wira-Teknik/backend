package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type AttachmentCategory string

const (
	AttachmentCategoryShipmentDelivery AttachmentCategory = "shipment_delivery"
	AttachmentCategoryShipmentReceived AttachmentCategory = "shipment_received"
	AttachmentCategoryInvoice          AttachmentCategory = "invoice"
	AttachmentCategoryPaymentProof     AttachmentCategory = "payment_proof"
	AttachmentCategoryBon              AttachmentCategory = "bon"
	AttachmentCategorySuratJalan       AttachmentCategory = "surat_jalan"
)

type FileType string

const (
	FileTypeImage FileType = "image"
	FileTypeVideo FileType = "video"
	FileTypePdf   FileType = "pdf"
)

type Attachment struct {
	ID        uuid.UUID          `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	RelatedID uuid.UUID          `gorm:"type:uuid;not null;index" json:"related_id"`
	Category  AttachmentCategory `gorm:"type:varchar(50);not null;check:category IN ('shipment_delivery', 'shipment_received', 'invoice', 'payment_proof', 'bon', 'surat_jalan')" json:"category"`
	FileType  FileType           `gorm:"type:varchar(20);not null;check:file_type IN ('image', 'video', 'pdf')" json:"file_type"`
	FileURL   string             `gorm:"type:varchar(255);not null" json:"file_url"`
	CreatedAt time.Time          `json:"created_at"`
	UpdatedAt time.Time          `json:"updated_at"`
	DeletedAt gorm.DeletedAt     `gorm:"index" json:"-"`
}
