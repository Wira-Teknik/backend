package services

import (
	"errors"
	"fmt"
	"time"

	"teknik/config"
	"teknik/models"
	"teknik/utils"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Sentinel errors for Shipment service
var (
	ErrShipmentInvalidUUID    = errors.New("ID pengiriman tidak valid")
	ErrShipmentInvalidOrderID = errors.New("ID pesanan tidak valid")
	ErrShipmentNotFound       = errors.New("pengiriman tidak ditemukan")
)

// ─────────────────────────────────────────────
// DTOs
// ─────────────────────────────────────────────

type ShipmentItemInput struct {
	OrderItemID string `json:"order_item_id"`
	ShippingQty int    `json:"shipping_qty"`
}

type CreateShipmentInput struct {
	OrderID      string              `json:"order_id"`
	ShippingDate string              `json:"shipping_date"` // format: "2006-01-02"
	Items        []ShipmentItemInput `json:"items"`
}

type UpdateShipmentItemsInput struct {
	Items []ShipmentItemInput `json:"items"`
}

// ─────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────

// GenerateInvoiceNo menghasilkan nomor invoice: INV-YYYYMMDD-XXXX.
func GenerateInvoiceNo(tx *gorm.DB) string {
	dateStr := time.Now().Format("20060102")
	prefix := fmt.Sprintf("INV-%s-", dateStr)

	var count int64
	tx.Model(&models.Invoice{}).
		Where("invoice_no LIKE ?", prefix+"%").
		Count(&count)

	return fmt.Sprintf("%s%04d", prefix, count+1)
}

// ─────────────────────────────────────────────
// Get Shipments by Order ID
// ─────────────────────────────────────────────

func GetShipmentsByOrderID(orderID string) ([]models.Shipment, error) {
	if _, err := uuid.Parse(orderID); err != nil {
		return nil, ErrShipmentInvalidOrderID
	}

	var shipments []models.Shipment
	err := config.DB.Preload("Items").
		Where("order_id = ?", orderID).
		Order("created_at DESC").
		Find(&shipments).Error
	if shipments == nil {
		shipments = []models.Shipment{}
	}
	return shipments, err
}

// ─────────────────────────────────────────────
// Get Shipment By ID
// ─────────────────────────────────────────────

func GetShipmentByID(id string) (models.Shipment, error) {
	if _, err := uuid.Parse(id); err != nil {
		return models.Shipment{}, ErrShipmentInvalidUUID
	}

	var shipment models.Shipment
	err := config.DB.Preload("Items").
		First(&shipment, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.Shipment{}, ErrShipmentNotFound
		}
		return models.Shipment{}, err
	}
	return shipment, nil
}

// ─────────────────────────────────────────────
// Create Shipment (Partial Shipment)
// ─────────────────────────────────────────────

func CreateShipment(input CreateShipmentInput, userID uuid.UUID) (models.Shipment, error) {
	if len(input.Items) == 0 {
		return models.Shipment{}, fmt.Errorf("pengiriman harus memiliki minimal 1 item")
	}

	orderID, err := uuid.Parse(input.OrderID)
	if err != nil {
		return models.Shipment{}, ErrShipmentInvalidOrderID
	}

	shippingDate, err := time.ParseInLocation("2006-01-02", input.ShippingDate, time.Local)
	if err != nil {
		return models.Shipment{}, fmt.Errorf("format tanggal tidak valid (gunakan YYYY-MM-DD)")
	}

	// Validasi order
	var order models.Order
	if err := config.DB.Preload("Items").First(&order, "id = ?", orderID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.Shipment{}, ErrOrderNotFound
		}
		return models.Shipment{}, err
	}

	if order.OrderStatus == models.OrderStatusShipped || order.OrderStatus == models.OrderStatusCompleted {
		return models.Shipment{}, fmt.Errorf("pesanan sudah selesai dikirim")
	}

	// Build map order items untuk lookup cepat
	orderItemMap := make(map[uuid.UUID]*models.OrderItem)
	for i := range order.Items {
		orderItemMap[order.Items[i].ID] = &order.Items[i]
	}

	shipmentID := uuid.New()
	var shipmentItems []models.ShipmentItem
	var invoiceTotalAmount float64

	for _, si := range input.Items {
		itemID, err := uuid.Parse(si.OrderItemID)
		if err != nil {
			return models.Shipment{}, fmt.Errorf("order item ID tidak valid: %s", si.OrderItemID)
		}

		orderItem, exists := orderItemMap[itemID]
		if !exists {
			return models.Shipment{}, fmt.Errorf("item dengan ID %s tidak ditemukan dalam pesanan ini", si.OrderItemID)
		}

		if si.ShippingQty <= 0 {
			return models.Shipment{}, fmt.Errorf("jumlah kirim harus lebih dari 0")
		}

		if si.ShippingQty > orderItem.RemainingQty {
			return models.Shipment{}, fmt.Errorf(
				"jumlah kirim (%d) melebihi sisa pesanan (%d) untuk produk %s",
				si.ShippingQty, orderItem.RemainingQty, orderItem.ProductName,
			)
		}

		shipmentItems = append(shipmentItems, models.ShipmentItem{
			ID:          uuid.New(),
			ShipmentID:  shipmentID,
			OrderItemID: itemID,
			ShippingQty: si.ShippingQty,
		})

		// Kalkulasi subtotal untuk invoice (unit_price * qty * 1.11)
		itemTotal := roundTwo(float64(si.ShippingQty) * orderItem.UnitPrice * (1 + ppnRate))
		invoiceTotalAmount += itemTotal
	}

	invoiceTotalAmount = roundTwo(invoiceTotalAmount)

	// Mulai transaksi database
	tx := config.DB.Begin()

	// 1. Buat shipment
	shipment := models.Shipment{
		ID:             shipmentID,
		OrderID:        orderID,
		ShippingDate:   utils.JSONDate(shippingDate),
		ShippingStatus: models.ShippingStatusDikirim,
		Items:          shipmentItems,
	}

	if err := tx.Create(&shipment).Error; err != nil {
		tx.Rollback()
		return models.Shipment{}, fmt.Errorf("gagal membuat pengiriman: %w", err)
	}

	// 2. Update remaining_qty di OrderItems
	for _, si := range input.Items {
		itemID, _ := uuid.Parse(si.OrderItemID)

		result := tx.Model(&models.OrderItem{}).
			Where("id = ? AND remaining_qty >= ?", itemID, si.ShippingQty).
			Update("remaining_qty", config.DB.Raw("remaining_qty - ?", si.ShippingQty))

		if result.Error != nil {
			tx.Rollback()
			return models.Shipment{}, fmt.Errorf("gagal memperbarui sisa qty: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			tx.Rollback()
			return models.Shipment{}, fmt.Errorf("konflik stok: sisa qty sudah berubah, silakan coba lagi")
		}
	}

	// 3. Update status order
	if err := updateOrderStatus(tx, orderID); err != nil {
		tx.Rollback()
		return models.Shipment{}, fmt.Errorf("gagal memperbarui status pesanan: %w", err)
	}

	// 4. Auto-generate Invoice
	var invoiceCreated bool
	var invoice models.Invoice

	invoice = models.Invoice{
		ID:               uuid.New(),
		ShipmentID:       shipmentID,
		InvoiceNo:        GenerateInvoiceNo(tx),
		TotalAmount:      invoiceTotalAmount,
		RemainingBalance: invoiceTotalAmount,
		PaymentStatus:    models.PaymentStatusUnpaid,
	}

	if err := tx.Create(&invoice).Error; err != nil {
		tx.Rollback()
		return models.Shipment{}, fmt.Errorf("gagal membuat invoice: %w", err)
	}
	invoiceCreated = true

	// Commit transaksi
	if err := tx.Commit().Error; err != nil {
		return models.Shipment{}, fmt.Errorf("gagal menyimpan data pengiriman: %w", err)
	}

	if invoiceCreated {
		shipment.Invoice = &invoice
	}

	// Audit log
	CreateAuditLog(userID, shipmentID, models.AuditActionCreate, "shipments", nil, shipment)
	if invoiceCreated {
		CreateAuditLog(userID, invoice.ID, models.AuditActionCreate, "invoices", nil, invoice)
	}

	return shipment, nil
}

// ─────────────────────────────────────────────
// Confirm Received (Atomic Transaction)
// ─────────────────────────────────────────────

func ConfirmShipmentReceived(id string, userID uuid.UUID) (models.Shipment, error) {
	if _, err := uuid.Parse(id); err != nil {
		return models.Shipment{}, ErrShipmentInvalidUUID
	}

	var shipment models.Shipment
	tx := config.DB.Begin()

	if err := tx.Preload("Items").First(&shipment, "id = ?", id).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.Shipment{}, ErrShipmentNotFound
		}
		return models.Shipment{}, err
	}

	if shipment.ShippingStatus == models.ShippingStatusDiterima {
		tx.Rollback()
		return models.Shipment{}, fmt.Errorf("pengiriman sudah dikonfirmasi diterima")
	}

	oldShipment := shipment

	now := utils.JSONDate(time.Now())
	shipment.ReceivedDate = &now
	shipment.ShippingStatus = models.ShippingStatusDiterima

	if err := tx.Save(&shipment).Error; err != nil {
		tx.Rollback()
		return models.Shipment{}, fmt.Errorf("gagal mengkonfirmasi penerimaan: %w", err)
	}

	// Cek apakah semua shipment untuk order ini sudah diterima
	var pendingCount int64
	if err := tx.Model(&models.Shipment{}).
		Where("order_id = ? AND shipping_status = ?", shipment.OrderID, models.ShippingStatusDikirim).
		Count(&pendingCount).Error; err != nil {
		tx.Rollback()
		return models.Shipment{}, fmt.Errorf("gagal memeriksa status pengiriman: %w", err)
	}

	// Jika semua shipment diterima DAN semua item terkirim, update ke completed
	if pendingCount == 0 {
		var order models.Order
		if err := tx.Preload("Items").First(&order, "id = ?", shipment.OrderID).Error; err == nil {
			allShipped := true
			for _, item := range order.Items {
				if item.RemainingQty > 0 {
					allShipped = false
					break
				}
			}
			if allShipped {
				if err := tx.Model(&models.Order{}).
					Where("id = ?", shipment.OrderID).
					Update("order_status", models.OrderStatusCompleted).Error; err != nil {
					tx.Rollback()
					return models.Shipment{}, fmt.Errorf("gagal memperbarui status pesanan: %w", err)
				}
			}
		}
	}

	if err := tx.Commit().Error; err != nil {
		return models.Shipment{}, fmt.Errorf("gagal menyimpan data penerimaan: %w", err)
	}

	CreateAuditLog(userID, shipment.ID, models.AuditActionUpdate, "shipments", oldShipment, shipment)

	return shipment, nil
}

// UpdateShipmentItems memperbarui daftar item dan kuantitas pengiriman yang sudah dibuat.
// Fungsi ini juga secara otomatis menyesuaikan remaining_qty di order_items serta total tagihan di invoices.
func UpdateShipmentItems(shipmentIDStr string, input UpdateShipmentItemsInput, userID uuid.UUID) (models.Shipment, error) {
	if len(input.Items) == 0 {
		return models.Shipment{}, fmt.Errorf("pengiriman harus memiliki minimal 1 item")
	}

	shipmentID, err := uuid.Parse(shipmentIDStr)
	if err != nil {
		return models.Shipment{}, ErrShipmentInvalidUUID
	}

	tx := config.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 1. Ambil shipment beserta items
	var shipment models.Shipment
	if err := tx.Preload("Items").First(&shipment, "id = ?", shipmentID).Error; err != nil {
		tx.Rollback()
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.Shipment{}, ErrShipmentNotFound
		}
		return models.Shipment{}, err
	}

	// 2. Cegah edit jika status sudah diterima
	if shipment.ShippingStatus == models.ShippingStatusDiterima {
		tx.Rollback()
		return models.Shipment{}, fmt.Errorf("pengiriman yang sudah diterima tidak dapat diedit")
	}

	// 3. Ambil data order asli beserta items
	var order models.Order
	if err := tx.Preload("Items").First(&order, "id = ?", shipment.OrderID).Error; err != nil {
		tx.Rollback()
		return models.Shipment{}, fmt.Errorf("gagal mengambil data pesanan terkait: %w", err)
	}

	// Build map order items untuk lookup cepat
	orderItemMap := make(map[uuid.UUID]*models.OrderItem)
	for i := range order.Items {
		orderItemMap[order.Items[i].ID] = &order.Items[i]
	}

	// Build map shipment items lama untuk lookup cepat
	oldShipmentItemsMap := make(map[uuid.UUID]models.ShipmentItem)
	for _, item := range shipment.Items {
		oldShipmentItemsMap[item.OrderItemID] = item
	}

	// Simpan salinan data lama untuk audit log
	var oldItemsCopy []models.ShipmentItem
	for _, item := range shipment.Items {
		oldItemsCopy = append(oldItemsCopy, item)
	}
	oldShipmentCopy := shipment
	oldShipmentCopy.Items = oldItemsCopy

	// 4. Proses input items dan hitung perbedaan kuantitas
	type itemUpdate struct {
		orderItemID uuid.UUID
		newQty      int
		oldQty      int
		diff        int
	}
	var updates []itemUpdate
	inputItemMap := make(map[uuid.UUID]bool)

	for _, inputItem := range input.Items {
		orderItemID, err := uuid.Parse(inputItem.OrderItemID)
		if err != nil {
			tx.Rollback()
			return models.Shipment{}, fmt.Errorf("order item ID tidak valid: %s", inputItem.OrderItemID)
		}
		inputItemMap[orderItemID] = true

		orderItem, exists := orderItemMap[orderItemID]
		if !exists {
			tx.Rollback()
			return models.Shipment{}, fmt.Errorf("item dengan ID %s tidak ditemukan dalam pesanan ini", inputItem.OrderItemID)
		}

		if inputItem.ShippingQty <= 0 {
			tx.Rollback()
			return models.Shipment{}, fmt.Errorf("jumlah kirim harus lebih dari 0")
		}

		oldItem, existed := oldShipmentItemsMap[orderItemID]
		oldQty := 0
		if existed {
			oldQty = oldItem.ShippingQty
		}

		diff := inputItem.ShippingQty - oldQty
		if diff > orderItem.RemainingQty {
			tx.Rollback()
			return models.Shipment{}, fmt.Errorf(
				"jumlah kirim baru (%d) melebihi sisa pesanan (%d) untuk produk %s",
				inputItem.ShippingQty, orderItem.RemainingQty+oldQty, orderItem.ProductName,
			)
		}

		updates = append(updates, itemUpdate{
			orderItemID: orderItemID,
			newQty:      inputItem.ShippingQty,
			oldQty:      oldQty,
			diff:        diff,
		})
	}

	// Tambahkan item lama yang tidak ada di input sebagai dihapus (newQty = 0)
	for orderItemID, oldItem := range oldShipmentItemsMap {
		if !inputItemMap[orderItemID] {
			updates = append(updates, itemUpdate{
				orderItemID: orderItemID,
				newQty:      0,
				oldQty:      oldItem.ShippingQty,
				diff:        -oldItem.ShippingQty,
			})
		}
	}

	// 5. Eksekusi update order_items (remaining_qty) dan shipment_items
	var newShipmentItems []models.ShipmentItem
	var newInvoiceTotalAmount float64

	for _, up := range updates {
		orderItem := orderItemMap[up.orderItemID]

		// Update remaining_qty di OrderItem
		newRemainingQty := orderItem.RemainingQty - up.diff
		if err := tx.Model(&models.OrderItem{}).
			Where("id = ?", up.orderItemID).
			Update("remaining_qty", newRemainingQty).Error; err != nil {
			tx.Rollback()
			return models.Shipment{}, fmt.Errorf("gagal memperbarui sisa qty pesanan: %w", err)
		}

		if up.newQty > 0 {
			// Item tetap ada atau baru
			var shipmentItem models.ShipmentItem
			oldItem, existed := oldShipmentItemsMap[up.orderItemID]
			if existed {
				shipmentItem = oldItem
				shipmentItem.ShippingQty = up.newQty
				if err := tx.Save(&shipmentItem).Error; err != nil {
					tx.Rollback()
					return models.Shipment{}, fmt.Errorf("gagal memperbarui item pengiriman: %w", err)
				}
			} else {
				shipmentItem = models.ShipmentItem{
					ID:          uuid.New(),
					ShipmentID:  shipmentID,
					OrderItemID: up.orderItemID,
					ShippingQty: up.newQty,
				}
				if err := tx.Create(&shipmentItem).Error; err != nil {
					tx.Rollback()
					return models.Shipment{}, fmt.Errorf("gagal membuat item pengiriman baru: %w", err)
				}
			}
			newShipmentItems = append(newShipmentItems, shipmentItem)

			// Hitung total nominal subtotal untuk invoice
			itemTotal := roundTwo(float64(up.newQty) * orderItem.UnitPrice * (1 + ppnRate))
			newInvoiceTotalAmount += itemTotal
		} else {
			// Item dihapus dari shipment
			oldItem := oldShipmentItemsMap[up.orderItemID]
			if err := tx.Delete(&oldItem).Error; err != nil {
				tx.Rollback()
				return models.Shipment{}, fmt.Errorf("gagal menghapus item pengiriman: %w", err)
			}
		}
	}

	newInvoiceTotalAmount = roundTwo(newInvoiceTotalAmount)

	// 6. Update Invoice terkait
	var invoice models.Invoice
	if err := tx.Where("shipment_id = ?", shipment.ID).First(&invoice).Error; err != nil {
		tx.Rollback()
		return models.Shipment{}, fmt.Errorf("gagal mengambil data invoice terkait: %w", err)
	}

	// Ambil jumlah pembayaran yang sudah dialokasikan pada invoice ini
	var totalPaid float64
	if err := tx.Model(&models.PaymentDetail{}).
		Where("invoice_id = ?", invoice.ID).
		Select("COALESCE(SUM(allocated_amount), 0)").
		Scan(&totalPaid).Error; err != nil {
		tx.Rollback()
		return models.Shipment{}, fmt.Errorf("gagal memeriksa riwayat alokasi pembayaran: %w", err)
	}

	if newInvoiceTotalAmount < totalPaid {
		tx.Rollback()
		return models.Shipment{}, fmt.Errorf(
			"kuantitas pengiriman tidak dapat dikurangi karena total tagihan baru (Rp %.2f) kurang dari jumlah yang telah dibayar (Rp %.2f)",
			newInvoiceTotalAmount, totalPaid,
		)
	}

	oldInvoiceCopy := invoice

	newRemainingBalance := roundTwo(newInvoiceTotalAmount - totalPaid)
	var newInvoiceStatus models.PaymentStatus
	if newRemainingBalance <= 0 {
		newInvoiceStatus = models.PaymentStatusPaid
		newRemainingBalance = 0
	} else if newRemainingBalance < newInvoiceTotalAmount {
		newInvoiceStatus = models.PaymentStatusPartial
	} else {
		newInvoiceStatus = models.PaymentStatusUnpaid
	}

	invoice.TotalAmount = newInvoiceTotalAmount
	invoice.RemainingBalance = newRemainingBalance
	invoice.PaymentStatus = newInvoiceStatus

	if err := tx.Save(&invoice).Error; err != nil {
		tx.Rollback()
		return models.Shipment{}, fmt.Errorf("gagal memperbarui invoice: %w", err)
	}

	// 7. Hitung ulang status order
	if err := updateOrderStatus(tx, shipment.OrderID); err != nil {
		tx.Rollback()
		return models.Shipment{}, fmt.Errorf("gagal memperbarui status pesanan: %w", err)
	}

	// Commit transaksi
	if err := tx.Commit().Error; err != nil {
		return models.Shipment{}, fmt.Errorf("gagal menyimpan pembaruan pengiriman: %w", err)
	}

	// Audit Logs
	shipment.Items = newShipmentItems
	CreateAuditLog(userID, shipment.ID, models.AuditActionUpdate, "shipments", oldShipmentCopy, shipment)
	CreateAuditLog(userID, invoice.ID, models.AuditActionUpdate, "invoices", oldInvoiceCopy, invoice)

	// Preload Invoice & Items untuk return value
	shipment.Invoice = &invoice

	return shipment, nil
}
