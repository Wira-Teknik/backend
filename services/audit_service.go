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

// GetAuditLogs mengambil daftar log audit berdasarkan pencarian nama admin.
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

// getFriendlyAction memetakan aksi audit mentah ke bentuk teks ramah pengguna.
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

// getStringFromJSON mengekstrak nilai string dari string JSON berdasarkan kunci.
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

// getFloatFromJSON mengekstrak nilai float64 dari string JSON berdasarkan kunci.
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

// generateAuditDescription menyusun deskripsi ramah pengguna secara otomatis untuk log audit.
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
		category := getStringFromJSON(logVal.NewValue, "category")
		categoryFriendly := ""
		switch category {
		case "shipment_delivery":
			categoryFriendly = "Bukti Pengiriman"
		case "shipment_received":
			categoryFriendly = "Bukti Penerimaan"
		case "invoice":
			categoryFriendly = "Invoice"
		case "payment_proof":
			categoryFriendly = "Bukti Pembayaran"
		case "bon":
			categoryFriendly = "Bon"
		case "surat_jalan":
			categoryFriendly = "Surat Jalan"
		default:
			categoryFriendly = "Dokumen Baru"
		}

		fileURL := getStringFromJSON(logVal.NewValue, "file_url")
		fileName := ""
		if fileURL != "" {
			for i := len(fileURL) - 1; i >= 0; i-- {
				if fileURL[i] == '/' || fileURL[i] == '\\' {
					fileName = fileURL[i+1:]
					break
				}
			}
			if fileName == "" {
				fileName = fileURL
			}
		}

		var docInfo string
		relatedID := getStringFromJSON(logVal.NewValue, "related_id")
		if relatedID != "" {
			switch category {
			case "shipment_delivery", "shipment_received":
				var info struct {
					RecipientName string
					TransactionNo string
				}
				config.DB.Table("shipments").
					Select("orders.recipient_name, orders.transaction_no").
					Joins("JOIN orders ON orders.id = shipments.order_id").
					Where("shipments.id = ?", relatedID).
					Scan(&info)
				if info.TransactionNo != "" {
					docInfo = fmt.Sprintf(" untuk Pengiriman Pesanan %s (%s)", info.RecipientName, info.TransactionNo)
				}
			case "invoice":
				var info struct {
					InvoiceNo     string
					RecipientName string
				}
				config.DB.Table("invoices").
					Select("invoices.invoice_no, orders.recipient_name").
					Joins("JOIN shipments ON shipments.id = invoices.shipment_id").
					Joins("JOIN orders ON orders.id = shipments.order_id").
					Where("invoices.id = ?", relatedID).
					Scan(&info)
				if info.InvoiceNo != "" {
					docInfo = fmt.Sprintf(" untuk Invoice %s (%s)", info.InvoiceNo, info.RecipientName)
				}
			case "payment_proof":
				var info struct {
					PaymentTotal float64
				}
				config.DB.Table("payments").
					Select("payment_total").
					Where("id = ?", relatedID).
					Scan(&info)
				if info.PaymentTotal > 0 {
					docInfo = fmt.Sprintf(" untuk Pembayaran Sebesar %s", formatRupiah(info.PaymentTotal))
				}
			}
		}

		if fileName != "" {
			return fmt.Sprintf("Admin Mengunggah Dokumen %s (%s)%s", categoryFriendly, fileName, docInfo)
		}
		return fmt.Sprintf("Admin Mengunggah Dokumen %s%s", categoryFriendly, docInfo)
	}

	target := ""
	switch logVal.TableName {
	case "orders":
		trxNo := getStringFromJSON(logVal.NewValue, "transaction_no")
		if trxNo == "" {
			trxNo = getStringFromJSON(logVal.OldValue, "transaction_no")
		}
		recipientName := getStringFromJSON(logVal.NewValue, "recipient_name")
		if recipientName == "" {
			recipientName = getStringFromJSON(logVal.OldValue, "recipient_name")
		}
		poNo := getStringFromJSON(logVal.NewValue, "po_no")
		if poNo == "" {
			poNo = getStringFromJSON(logVal.OldValue, "po_no")
		}

		if trxNo != "" {
			if poNo != "" {
				target = fmt.Sprintf("Pemesanan %s dengan No Transaksi %s (PO: %s)", recipientName, trxNo, poNo)
			} else {
				target = fmt.Sprintf("Pemesanan %s dengan No Transaksi %s", recipientName, trxNo)
			}
		} else {
			target = "Pemesanan"
		}
	case "payments":
		type paymentAllocInfo struct {
			InvoiceNo       string
			RecipientName   string
			TransactionNo   string
			PoNo            string
			AllocatedAmount float64
		}
		var allocs []paymentAllocInfo
		config.DB.Table("payment_details").
			Select("invoices.invoice_no, orders.recipient_name, orders.transaction_no, orders.po_no, payment_details.allocated_amount").
			Joins("JOIN invoices ON invoices.id = payment_details.invoice_id").
			Joins("JOIN shipments ON shipments.id = invoices.shipment_id").
			Joins("JOIN orders ON orders.id = shipments.order_id").
			Where("payment_details.payment_id = ?", logVal.ResourceID).
			Scan(&allocs)

		total := getFloatFromJSON(logVal.NewValue, "payment_total")
		if total == 0 {
			total = getFloatFromJSON(logVal.OldValue, "payment_total")
		}

		if len(allocs) > 0 {
			formattedTotal := formatRupiah(total)
			if len(allocs) == 1 {
				alloc := allocs[0]
				var orderDetail string
				if alloc.PoNo != "" {
					orderDetail = fmt.Sprintf(" untuk Invoice %s (Pesanan %s - %s [PO: %s])", alloc.InvoiceNo, alloc.RecipientName, alloc.TransactionNo, alloc.PoNo)
				} else {
					orderDetail = fmt.Sprintf(" untuk Invoice %s (Pesanan %s - %s)", alloc.InvoiceNo, alloc.RecipientName, alloc.TransactionNo)
				}
				target = fmt.Sprintf("Pembayaran Sebesar %s%s", formattedTotal, orderDetail)
			} else {
				customersMap := make(map[string]bool)
				for _, a := range allocs {
					customersMap[a.RecipientName] = true
				}
				var customers []string
				for name := range customersMap {
					customers = append(customers, name)
				}
				
				custStr := ""
				if len(customers) == 1 {
					custStr = customers[0]
				} else if len(customers) == 2 {
					custStr = fmt.Sprintf("%s dan %s", customers[0], customers[1])
				} else {
					custStr = fmt.Sprintf("%s, %s, dan lainnya", customers[0], customers[1])
				}
				target = fmt.Sprintf("Pembayaran Sebesar %s untuk %d Invoice (%s)", formattedTotal, len(allocs), custStr)
			}
		} else if total > 0 {
			target = fmt.Sprintf("Pembayaran Sebesar %s", formatRupiah(total))
		} else {
			target = "Pembayaran"
		}
	case "shipments":
		type shipmentOrderInfo struct {
			RecipientName string
			TransactionNo string
			PoNo          string
		}
		var info shipmentOrderInfo
		config.DB.Table("shipments").
			Select("orders.recipient_name, orders.transaction_no, orders.po_no").
			Joins("JOIN orders ON orders.id = shipments.order_id").
			Where("shipments.id = ?", logVal.ResourceID).
			Scan(&info)

		if info.TransactionNo != "" {
			var orderDetail string
			if info.PoNo != "" {
				orderDetail = fmt.Sprintf(" untuk Pesanan %s dengan No Transaksi %s (PO: %s)", info.RecipientName, info.TransactionNo, info.PoNo)
			} else {
				orderDetail = fmt.Sprintf(" untuk Pesanan %s dengan No Transaksi %s", info.RecipientName, info.TransactionNo)
			}
			target = "Pengiriman" + orderDetail
		} else {
			target = "Pengiriman"
		}
	case "invoices":
		type invoiceOrderInfo struct {
			RecipientName string
			TransactionNo string
			PoNo          string
			InvoiceNo     string
			TotalAmount   float64
		}
		var info invoiceOrderInfo
		config.DB.Table("invoices").
			Select("orders.recipient_name, orders.transaction_no, orders.po_no, invoices.invoice_no, invoices.total_amount").
			Joins("JOIN shipments ON shipments.id = invoices.shipment_id").
			Joins("JOIN orders ON orders.id = shipments.order_id").
			Where("invoices.id = ?", logVal.ResourceID).
			Scan(&info)

		if info.InvoiceNo != "" {
			formattedAmount := formatRupiah(info.TotalAmount)
			var orderDetail string
			if info.PoNo != "" {
				orderDetail = fmt.Sprintf(" Sebesar %s untuk Pesanan %s (No Transaksi: %s, PO: %s)", formattedAmount, info.RecipientName, info.TransactionNo, info.PoNo)
			} else {
				orderDetail = fmt.Sprintf(" Sebesar %s untuk Pesanan %s (No Transaksi: %s)", formattedAmount, info.RecipientName, info.TransactionNo)
			}
			target = "Tagihan " + info.InvoiceNo + orderDetail
		} else {
			invNo := getStringFromJSON(logVal.NewValue, "invoice_no")
			if invNo == "" {
				invNo = getStringFromJSON(logVal.OldValue, "invoice_no")
			}
			if invNo != "" {
				target = "Tagihan " + invNo
			} else {
				target = "Tagihan"
			}
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
