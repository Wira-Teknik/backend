package services

import (
	"fmt"
	"time"

	"teknik/config"
	"teknik/models"

	"github.com/google/uuid"
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

// ─────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────

// generateInvoiceNo menghasilkan nomor invoice: INV-YYYYMMDD-XXXX.
func generateInvoiceNo() string {
	dateStr := time.Now().Format("20060102")
	prefix := fmt.Sprintf("INV-%s-", dateStr)

	var count int64
	config.DB.Model(&models.Invoice{}).
		Where("invoice_no LIKE ?", prefix+"%").
		Count(&count)

	return fmt.Sprintf("%s%04d", prefix, count+1)
}

// ─────────────────────────────────────────────
// Get Shipments by Order ID
// ─────────────────────────────────────────────

func GetShipmentsByOrderID(orderID string) ([]models.Shipment, error) {
	var shipments []models.Shipment
	err := config.DB.Preload("Items").
		Where("order_id = ?", orderID).
		Order("created_at DESC").
		Find(&shipments).Error
	return shipments, err
}

// ─────────────────────────────────────────────
// Get Shipment By ID
// ─────────────────────────────────────────────

func GetShipmentByID(id string) (models.Shipment, error) {
	var shipment models.Shipment
	err := config.DB.Preload("Items").
		First(&shipment, "id = ?", id).Error
	return shipment, err
}

// ─────────────────────────────────────────────
// Create Shipment (Partial Shipment)
// ─────────────────────────────────────────────
// Alur:
// 1. Validasi order exists dan status bukan 'shipped'/'completed'
// 2. Validasi shipping_qty <= remaining_qty per item
// 3. Buat Shipment + ShipmentItems dalam transaksi
// 4. Kurangi remaining_qty di OrderItems
// 5. Update status order (partial/shipped)
// 6. Auto-generate Invoice

func CreateShipment(input CreateShipmentInput, userID uuid.UUID) (models.Shipment, error) {
	if len(input.Items) == 0 {
		return models.Shipment{}, fmt.Errorf("pengiriman harus memiliki minimal 1 item")
	}

	orderID, err := uuid.Parse(input.OrderID)
	if err != nil {
		return models.Shipment{}, fmt.Errorf("order ID tidak valid")
	}

	shippingDate, err := time.Parse("2006-01-02", input.ShippingDate)
	if err != nil {
		return models.Shipment{}, fmt.Errorf("format tanggal tidak valid (gunakan YYYY-MM-DD)")
	}

	// Validasi order
	var order models.Order
	if err := config.DB.Preload("Items").First(&order, "id = ?", orderID).Error; err != nil {
		return models.Shipment{}, fmt.Errorf("pesanan tidak ditemukan")
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
		ShippingDate:   shippingDate,
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
	invoice := models.Invoice{
		ID:               uuid.New(),
		ShipmentID:       shipmentID,
		InvoiceNo:        generateInvoiceNo(),
		TotalAmount:      invoiceTotalAmount,
		RemainingBalance: invoiceTotalAmount,
		PaymentStatus:    models.PaymentStatusUnpaid,
	}

	if err := tx.Create(&invoice).Error; err != nil {
		tx.Rollback()
		return models.Shipment{}, fmt.Errorf("gagal membuat invoice: %w", err)
	}

	// Commit transaksi
	if err := tx.Commit().Error; err != nil {
		return models.Shipment{}, fmt.Errorf("gagal menyimpan data pengiriman: %w", err)
	}

	// Audit log
	CreateAuditLog(userID, shipmentID, models.AuditActionCreate, "shipments", nil, shipment)
	CreateAuditLog(userID, invoice.ID, models.AuditActionCreate, "invoices", nil, invoice)

	return shipment, nil
}

// ─────────────────────────────────────────────
// Confirm Received
// ─────────────────────────────────────────────

func ConfirmShipmentReceived(id string, userID uuid.UUID) (models.Shipment, error) {
	var shipment models.Shipment
	if err := config.DB.Preload("Items").First(&shipment, "id = ?", id).Error; err != nil {
		return models.Shipment{}, fmt.Errorf("pengiriman tidak ditemukan")
	}

	if shipment.ShippingStatus == models.ShippingStatusDiterima {
		return models.Shipment{}, fmt.Errorf("pengiriman sudah dikonfirmasi diterima")
	}

	oldShipment := shipment

	now := time.Now()
	shipment.ReceivedDate = &now
	shipment.ShippingStatus = models.ShippingStatusDiterima

	if err := config.DB.Save(&shipment).Error; err != nil {
		return models.Shipment{}, fmt.Errorf("gagal mengkonfirmasi penerimaan")
	}

	// Cek apakah semua shipment untuk order ini sudah diterima
	var pendingCount int64
	config.DB.Model(&models.Shipment{}).
		Where("order_id = ? AND shipping_status = ?", shipment.OrderID, models.ShippingStatusDikirim).
		Count(&pendingCount)

	// Jika semua shipment diterima DAN semua item terkirim, update ke completed
	if pendingCount == 0 {
		var order models.Order
		if err := config.DB.Preload("Items").First(&order, "id = ?", shipment.OrderID).Error; err == nil {
			allShipped := true
			for _, item := range order.Items {
				if item.RemainingQty > 0 {
					allShipped = false
					break
				}
			}
			if allShipped {
				config.DB.Model(&models.Order{}).
					Where("id = ?", shipment.OrderID).
					Update("order_status", models.OrderStatusCompleted)
			}
		}
	}

	CreateAuditLog(userID, shipment.ID, models.AuditActionUpdate, "shipments", oldShipment, shipment)

	return shipment, nil
}
