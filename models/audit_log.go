package models

import (
	"github.com/google/uuid"
	"teknik/utils"
)

// AuditAction mendefinisikan jenis tindakan audit yang direkam.
type AuditAction string

const (
	AuditActionCreate    AuditAction = "CREATE"
	AuditActionUpdate    AuditAction = "UPDATE"
	AuditActionDelete    AuditAction = "DELETE"
	AuditActionLogin     AuditAction = "LOGIN"
	AuditActionUploadDoc AuditAction = "UPLOAD_DOC"
)

// AuditLog merepresentasikan catatan audit untuk melacak aktivitas pengguna dalam sistem.
type AuditLog struct {
	ID         uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID     uuid.UUID      `gorm:"type:uuid;not null;index" json:"user_id"`
	User       User           `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;" json:"user"`
	Action     AuditAction    `gorm:"type:varchar(20);not null;check:action IN ('CREATE', 'UPDATE', 'DELETE', 'LOGIN', 'UPLOAD_DOC')" json:"action"`
	TableName  string         `gorm:"type:varchar(255)" json:"table_name"`
	ResourceID uuid.UUID      `gorm:"type:uuid;index" json:"resource_id"`
	OldValue   string         `gorm:"type:json" json:"old_value"`
	NewValue   string         `gorm:"type:json" json:"new_value"`
	CreatedAt  utils.JSONDateTime `json:"created_at"`
}
