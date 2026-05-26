package services

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"teknik/config"
	"teknik/models"

	"github.com/google/uuid"
)

// CreateAuditLog mencatat perubahan data ke tabel audit_logs.
// oldData dan newData bisa nil jika tidak relevan (misal: DELETE hanya perlu oldData, CREATE hanya newData).
func CreateAuditLog(userID, resourceID uuid.UUID, action models.AuditAction, tableName string, oldData, newData interface{}) {
	var oldJSON, newJSON string = "null", "null"

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

type AuditLogDTO struct {
	ID          string `json:"id"`
	AdminName   string `json:"admin_name"`
	Action      string `json:"action"`
	ActionRaw   string `json:"action_raw"`
	Date        string `json:"date"`
	Description string `json:"description"`
}

func GetAuditLogs(searchAdminName string) ([]AuditLogDTO, error) {
	var logs []models.AuditLog

	query := config.DB.Preload("User").
		Joins("LEFT JOIN users ON users.id = audit_logs.user_id AND users.deleted_at IS NULL").
		Order("audit_logs.created_at DESC")

	if searchAdminName != "" {
		query = query.Where("users.name ILIKE ?", "%"+searchAdminName+"%")
	}

	if err := query.Find(&logs).Error; err != nil {
		return nil, err
	}

	dtos := []AuditLogDTO{}
	for _, logVal := range logs {
		adminName := "Unknown Admin"
		if logVal.User.ID != uuid.Nil {
			adminName = logVal.User.Name
		}

		t := time.Time(logVal.CreatedAt).Local()
		formattedDate := t.Format("2006-01-02 15:04")

		dtos = append(dtos, AuditLogDTO{
			ID:          logVal.ID.String(),
			AdminName:   adminName,
			Action:      getFriendlyAction(logVal.Action),
			ActionRaw:   string(logVal.Action),
			Date:        formattedDate,
			Description: generateAuditDescription(logVal),
		})
	}

	return dtos, nil
}

func getFriendlyAction(action models.AuditAction) string {
	switch action {
	case models.AuditActionCreate:
		return "Create"
	case models.AuditActionUpdate:
		return "Update"
	case models.AuditActionDelete:
		return "Delete"
	case models.AuditActionLogin:
		return "Login"
	case models.AuditActionUploadDoc:
		return "Upload"
	default:
		return string(action)
	}
}

func getStringFromJSON(jsonStr string, keys ...string) string {
	if jsonStr == "" || jsonStr == "null" {
		return ""
	}
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return ""
	}
	for _, key := range keys {
		if val, exists := data[key]; exists {
			if strVal, ok := val.(string); ok {
				return strVal
			}
			if numVal, ok := val.(float64); ok {
				return fmt.Sprintf("%.0f", numVal)
			}
		}
	}
	return ""
}

func getFloatFromJSON(jsonStr string, key string) float64 {
	if jsonStr == "" || jsonStr == "null" {
		return 0
	}
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return 0
	}
	if val, exists := data[key]; exists {
		if numVal, ok := val.(float64); ok {
			return numVal
		}
	}
	return 0
}

func generateAuditDescription(logVal models.AuditLog) string {
	actionWord := ""
	switch logVal.Action {
	case models.AuditActionCreate:
		actionWord = "Menambahkan"
	case models.AuditActionUpdate:
		actionWord = "Mengupdate"
	case models.AuditActionDelete:
		actionWord = "Menghapus"
	case models.AuditActionLogin:
		return "Admin Login ke Sistem"
	case models.AuditActionUploadDoc:
		fileName := getStringFromJSON(logVal.NewValue, "file_name", "filename")
		if fileName == "" {
			fileName = "Dokumen Baru"
		}
		return "Admin Mengunggah Dokumen " + fileName
	}

	target := ""
	switch logVal.TableName {
	case "orders":
		trxNo := getStringFromJSON(logVal.NewValue, "transaction_no")
		if trxNo == "" {
			trxNo = getStringFromJSON(logVal.OldValue, "transaction_no")
		}
		if trxNo != "" {
			target = "Pemesanan " + trxNo
		} else {
			target = "Pemesanan"
		}
	case "payments":
		total := getFloatFromJSON(logVal.NewValue, "payment_total")
		if total == 0 {
			total = getFloatFromJSON(logVal.OldValue, "payment_total")
		}
		if total > 0 {
			target = fmt.Sprintf("Pembayaran Sebesar Rp %.0f", total)
		} else {
			target = "Pembayaran"
		}
	case "shipments":
		shipmentNo := getStringFromJSON(logVal.NewValue, "shipment_no")
		if shipmentNo == "" {
			shipmentNo = getStringFromJSON(logVal.OldValue, "shipment_no")
		}
		if shipmentNo != "" {
			target = "Pengiriman " + shipmentNo
		} else {
			target = "Pengiriman"
		}
	case "invoices":
		invNo := getStringFromJSON(logVal.NewValue, "invoice_no")
		if invNo == "" {
			invNo = getStringFromJSON(logVal.OldValue, "invoice_no")
		}
		if invNo != "" {
			target = "Tagihan " + invNo
		} else {
			target = "Tagihan"
		}
	case "customers":
		custName := getStringFromJSON(logVal.NewValue, "customer_name")
		if custName == "" {
			custName = getStringFromJSON(logVal.OldValue, "customer_name")
		}
		if custName != "" {
			target = "Customer " + custName
		} else {
			target = "Customer"
		}
	default:
		target = logVal.TableName
	}

	return fmt.Sprintf("Admin %s %s", actionWord, target)
}
