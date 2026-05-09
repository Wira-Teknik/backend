package services

import (
	"fmt"
	"math"
	"strings"
	"time"

	"teknik/config"
	"teknik/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ─────────────────────────────────────────────
// DTOs
// ─────────────────────────────────────────────

type OrderItemInput struct {
	ProductName string  `json:"product_name"`
	OrderQty    int     `json:"order_qty"`
	UnitPrice   float64 `json:"unit_price"`
}

type CreateOrderInput struct {
	PoNo             string           `json:"po_no"`
	OrderDate        string           `json:"order_date"` // format: "2006-01-02"
	RecipientName    string           `json:"recipient_name"`
	RecipientAddress string           `json:"recipient_address"`
	RecipientPhone   string           `json:"recipient_phone"`
	RecipientEmail   string           `json:"recipient_email"`
	Items            []OrderItemInput `json:"items"`
}

type UpdateOrderInput struct {
	PoNo             string `json:"po_no"`
	RecipientName    string `json:"recipient_name"`
	RecipientAddress string `json:"recipient_address"`
	RecipientPhone   string `json:"recipient_phone"`
	RecipientEmail   string `json:"recipient_email"`
}

// ─────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────

const ppnRate = 0.11

// generateTransactionNo menghasilkan nomor transaksi unik: TRX-YYYYMMDD-XXXX.
func generateTransactionNo() string {
	dateStr := time.Now().Format("20060102")
	prefix := fmt.Sprintf("TRX-%s-", dateStr)

	var count int64
	config.DB.Model(&models.Order{}).
		Where("transaction_no LIKE ?", prefix+"%").
		Count(&count)

	return fmt.Sprintf("%s%04d", prefix, count+1)
}

// roundTwo pembulatan 2 desimal.
func roundTwo(val float64) float64 {
	return math.Round(val*100) / 100
}

// ─────────────────────────────────────────────
// Get All Orders
// ─────────────────────────────────────────────

func GetAllOrders() ([]models.Order, error) {
	var orders []models.Order
	err := config.DB.Preload("Items").
		Order("created_at DESC").
		Find(&orders).Error
	return orders, err
}

// ─────────────────────────────────────────────
// Get Order By ID
// ─────────────────────────────────────────────

func GetOrderByID(id string) (models.Order, error) {
	var order models.Order
	err := config.DB.Preload("Items").
		Preload("Shipments").
		Preload("Shipments.Items").
		First(&order, "id = ?", id).Error
	return order, err
}

// ─────────────────────────────────────────────
// Create Order (with Items)
// ─────────────────────────────────────────────

func CreateOrder(input CreateOrderInput, userID uuid.UUID) (models.Order, error) {
	input.RecipientName = strings.TrimSpace(input.RecipientName)

	if input.RecipientName == "" {
		return models.Order{}, fmt.Errorf("nama penerima tidak boleh kosong")
	}
	if len(input.Items) == 0 {
		return models.Order{}, fmt.Errorf("pesanan harus memiliki minimal 1 item")
	}

	orderDate, err := time.Parse("2006-01-02", input.OrderDate)
	if err != nil {
		return models.Order{}, fmt.Errorf("format tanggal tidak valid (gunakan YYYY-MM-DD)")
	}

	orderID := uuid.New()
	var items []models.OrderItem

	for _, item := range input.Items {
		if strings.TrimSpace(item.ProductName) == "" {
			return models.Order{}, fmt.Errorf("nama produk tidak boleh kosong")
		}
		if item.OrderQty <= 0 {
			return models.Order{}, fmt.Errorf("jumlah order harus lebih dari 0")
		}
		if item.UnitPrice <= 0 {
			return models.Order{}, fmt.Errorf("harga satuan harus lebih dari 0")
		}

		ppn := roundTwo(float64(item.OrderQty) * item.UnitPrice * ppnRate)
		subtotal := roundTwo(float64(item.OrderQty)*item.UnitPrice + ppn)

		items = append(items, models.OrderItem{
			ID:           uuid.New(),
			OrderID:      orderID,
			ProductName:  strings.TrimSpace(item.ProductName),
			OrderQty:     item.OrderQty,
			RemainingQty: item.OrderQty, // saat awal, remaining = order qty
			UnitPrice:    item.UnitPrice,
			PPN:          ppn,
			Subtotal:     subtotal,
		})
	}

	order := models.Order{
		ID:               orderID,
		TransactionNo:    generateTransactionNo(),
		PoNo:             strings.TrimSpace(input.PoNo),
		OrderDate:        orderDate,
		RecipientName:    input.RecipientName,
		RecipientAddress: strings.TrimSpace(input.RecipientAddress),
		RecipientPhone:   strings.TrimSpace(input.RecipientPhone),
		RecipientEmail:   strings.TrimSpace(input.RecipientEmail),
		OrderStatus:      models.OrderStatusPending,
		Items:            items,
	}

	if err := config.DB.Create(&order).Error; err != nil {
		return models.Order{}, fmt.Errorf("gagal membuat pesanan: %w", err)
	}

	// Audit log
	CreateAuditLog(userID, orderID, models.AuditActionCreate, "orders", nil, order)

	return order, nil
}

// ─────────────────────────────────────────────
// Update Order (header only, status must be pending)
// ─────────────────────────────────────────────

func UpdateOrder(id string, input UpdateOrderInput, userID uuid.UUID) (models.Order, error) {
	order, err := GetOrderByID(id)
	if err != nil {
		return models.Order{}, fmt.Errorf("pesanan tidak ditemukan")
	}

	if order.OrderStatus != models.OrderStatusPending {
		return models.Order{}, fmt.Errorf("pesanan yang sudah diproses tidak dapat diubah")
	}

	oldOrder := order // snapshot untuk audit

	order.PoNo = strings.TrimSpace(input.PoNo)
	order.RecipientName = strings.TrimSpace(input.RecipientName)
	order.RecipientAddress = strings.TrimSpace(input.RecipientAddress)
	order.RecipientPhone = strings.TrimSpace(input.RecipientPhone)
	order.RecipientEmail = strings.TrimSpace(input.RecipientEmail)

	if err := config.DB.Save(&order).Error; err != nil {
		return models.Order{}, fmt.Errorf("gagal mengupdate pesanan")
	}

	CreateAuditLog(userID, order.ID, models.AuditActionUpdate, "orders", oldOrder, order)

	return order, nil
}

// ─────────────────────────────────────────────
// Delete Order (only if pending)
// ─────────────────────────────────────────────

func DeleteOrder(id string, userID uuid.UUID) error {
	order, err := GetOrderByID(id)
	if err != nil {
		return fmt.Errorf("pesanan tidak ditemukan")
	}

	if order.OrderStatus != models.OrderStatusPending {
		return fmt.Errorf("pesanan yang sudah diproses tidak dapat dihapus")
	}

	if err := config.DB.Select("Items").Delete(&order).Error; err != nil {
		return fmt.Errorf("gagal menghapus pesanan")
	}

	CreateAuditLog(userID, order.ID, models.AuditActionDelete, "orders", order, nil)

	return nil
}

// ─────────────────────────────────────────────
// updateOrderStatus menghitung ulang status order
// berdasarkan remaining_qty semua item.
// ─────────────────────────────────────────────

func updateOrderStatus(tx *gorm.DB, orderID uuid.UUID) error {
	var items []models.OrderItem
	if err := tx.Where("order_id = ?", orderID).Find(&items).Error; err != nil {
		return err
	}

	allShipped := true
	anyShipped := false

	for _, item := range items {
		if item.RemainingQty > 0 {
			allShipped = false
		}
		if item.RemainingQty < item.OrderQty {
			anyShipped = true
		}
	}

	var newStatus models.OrderStatus
	switch {
	case allShipped:
		newStatus = models.OrderStatusShipped
	case anyShipped:
		newStatus = models.OrderStatusPartial
	default:
		newStatus = models.OrderStatusPending
	}

	return tx.Model(&models.Order{}).Where("id = ?", orderID).Update("order_status", newStatus).Error
}
