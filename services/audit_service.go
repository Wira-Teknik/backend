package services

import (
	"encoding/json"
	"fmt"
	"log"

	"teknik/config"
	"teknik/models"

	"github.com/google/uuid"
)

// CreateAuditLog mencatat perubahan data ke tabel audit_logs.
// oldData dan newData bisa nil jika tidak relevan (misal: DELETE hanya perlu oldData, CREATE hanya newData).
func CreateAuditLog(userID, resourceID uuid.UUID, action models.AuditAction, tableName string, oldData, newData interface{}) {
	var oldJSON, newJSON string

	if oldData != nil {
		b, err := json.Marshal(oldData)
		if err == nil {
			oldJSON = string(b)
		}
	}
	if newData != nil {
		b, err := json.Marshal(newData)
		if err == nil {
			newJSON = string(b)
		}
	}

	auditLog := models.AuditLog{
		ID:         uuid.New(),
		UserID:     userID,
		Action:     action,
		TableName:  tableName,
		ResourceID: resourceID,
		OldValue:   oldJSON,
		NewValue:   newJSON,
	}

	if err := config.DB.Create(&auditLog).Error; err != nil {
		log.Printf("[audit] gagal menyimpan audit log: %v", err)
	}
}

// ParseUserID mengambil userID dari fiber locals dan mengembalikan uuid.UUID.
func ParseUserID(userIDStr string) (uuid.UUID, error) {
	id, err := uuid.Parse(userIDStr)
	if err != nil {
		return uuid.Nil, fmt.Errorf("user ID tidak valid")
	}
	return id, nil
}
