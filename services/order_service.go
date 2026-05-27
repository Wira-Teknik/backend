package services

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"teknik/config"
	"teknik/models"
	"teknik/utils"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Sentinel errors for Order service
var (
	ErrOrderInvalidUUID            = errors.New("ID pesanan tidak valid")
	ErrOrderNotFound               = errors.New("pesanan tidak ditemukan")
	ErrOrderDuplicateTransactionNo = errors.New("nomor transaksi sudah terdaftar")
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
	TransactionNo    string           `json:"transaction_no"`
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

// GenerateTransactionNo menghasilkan nomor transaksi unik: NF/WT/<banyaknya_order_tahun_ini>/<tahun>
func GenerateTransactionNo() string {
	year := time.Now().Year()

	var count int64
	startOfYear := time.Date(year, time.January, 1, 0, 0, 0, 0, time.Local)
	endOfYear := time.Date(year, time.December, 31, 23, 59, 59, 999999999, time.Local)

	config.DB.Unscoped().Model(&models.Order{}).
		Where("created_at >= ? AND created_at <= ?", startOfYear, endOfYear).
		Count(&count)

	return fmt.Sprintf("NF/WT/%d/%d", count+1, year)
}

// roundTwo pembulatan 2 desimal.
func roundTwo(val float64) float64 {
	return math.Round(val*100) / 100
}

// ─────────────────────────────────────────────
// Get All Orders
// ─────────────────────────────────────────────

// computeOrderPaymentInfo menghitung status pembayaran, total, dan sisa tagihan untuk suatu pesanan.
func computeOrderPaymentInfo(order *models.Order) {
	var totalOrderAmount float64

	// 1. Hitung total biaya keseluruhan pesanan dari items (Subtotal sudah termasuk PPN)
	for _, item := range order.Items {
		totalOrderAmount += item.Subtotal
	}
	totalOrderAmount = roundTwo(totalOrderAmount)

	// 2. Hitung total yang sudah dibayar dari invoice yang telah di-generate
	var totalPaid float64
	for _, shipment := range order.Shipments {
		if shipment.Invoice != nil {
			paidForInvoice := roundTwo(shipment.Invoice.TotalAmount - shipment.Invoice.RemainingBalance)
			totalPaid += paidForInvoice
		}
	}
	totalPaid = roundTwo(totalPaid)

	// 3. Hitung sisa yang harus dibayar untuk keseluruhan pesanan
	remaining := roundTwo(totalOrderAmount - totalPaid)

	order.TotalAmountToPay = totalOrderAmount
	order.RemainingBalance = remaining

	if totalOrderAmount == 0 {
		order.PaymentStatus = models.PaymentStatusUnpaid
	} else if remaining <= 0 {
		order.PaymentStatus = models.PaymentStatusPaid
		order.RemainingBalance = 0 // cegah nilai negatif
	} else if remaining < totalOrderAmount {
		order.PaymentStatus = models.PaymentStatusPartial
	} else {
		order.PaymentStatus = models.PaymentStatusUnpaid
	}
}

func GetAllOrders() ([]models.Order, error) {
	var orders []models.Order
	err := config.DB.Preload("Items").
		Preload("Shipments").
		Preload("Shipments.Items").
		Preload("Shipments.Invoice").
		Order("created_at DESC").
		Find(&orders).Error
	
	if err == nil {
		for i := range orders {
			computeOrderPaymentInfo(&orders[i])
		}
	}
	return orders, err
}

// ─────────────────────────────────────────────
// Get Order By ID
// ─────────────────────────────────────────────

func GetOrderByID(id string) (models.Order, error) {
	if _, err := uuid.Parse(id); err != nil {
		return models.Order{}, ErrOrderInvalidUUID
	}

	var order models.Order
	err := config.DB.Preload("Items").
		Preload("Shipments").
		Preload("Shipments.Items").
		Preload("Shipments.Invoice").
		First(&order, "id = ?", id).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.Order{}, ErrOrderNotFound
		}
		return models.Order{}, err
	}

	computeOrderPaymentInfo(&order)
	return order, nil
}

// ─────────────────────────────────────────────
// Create Order (with Items)
// ─────────────────────────────────────────────

func CreateOrder(input CreateOrderInput, userID uuid.UUID) (models.Order, error) {
	input.RecipientName = strings.TrimSpace(input.RecipientName)
	input.TransactionNo = strings.TrimSpace(input.TransactionNo)
	input.RecipientPhone = strings.TrimSpace(input.RecipientPhone)
	input.RecipientAddress = strings.TrimSpace(input.RecipientAddress)
	input.RecipientEmail = strings.TrimSpace(strings.ToLower(input.RecipientEmail))

	if input.TransactionNo == "" {
		return models.Order{}, fmt.Errorf("nomor transaksi tidak boleh kosong")
	}
	if input.RecipientName == "" {
		return models.Order{}, fmt.Errorf("nama penerima tidak boleh kosong")
	}
	if len(input.Items) == 0 {
		return models.Order{}, fmt.Errorf("pesanan harus memiliki minimal 1 item")
	}

	if input.RecipientEmail != "" && !utils.IsValidEmail(input.RecipientEmail) {
		return models.Order{}, fmt.Errorf("format email penerima tidak valid")
	}

	// Cek preemptif apakah nomor transaksi sudah terdaftar
	var count int64
	if err := config.DB.Model(&models.Order{}).Where("transaction_no = ?", input.TransactionNo).Count(&count).Error; err != nil {
		return models.Order{}, err
	}
	if count > 0 {
		return models.Order{}, ErrOrderDuplicateTransactionNo
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
		TransactionNo:    input.TransactionNo,
		PoNo:             strings.TrimSpace(input.PoNo),
		OrderDate:        utils.JSONDate(orderDate),
		RecipientName:    input.RecipientName,
		RecipientAddress: input.RecipientAddress,
		RecipientPhone:   input.RecipientPhone,
		RecipientEmail:   input.RecipientEmail,
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
		return models.Order{}, err
	}

	if order.OrderStatus != models.OrderStatusPending {
		return models.Order{}, fmt.Errorf("pesanan yang sudah diproses tidak dapat diubah")
	}

	input.RecipientName = strings.TrimSpace(input.RecipientName)
	if input.RecipientName == "" {
		return models.Order{}, fmt.Errorf("nama penerima tidak boleh kosong")
	}

	input.RecipientEmail = strings.TrimSpace(strings.ToLower(input.RecipientEmail))
	if input.RecipientEmail != "" && !utils.IsValidEmail(input.RecipientEmail) {
		return models.Order{}, fmt.Errorf("format email penerima tidak valid")
	}

	oldOrder := order // snapshot untuk audit

	order.PoNo = strings.TrimSpace(input.PoNo)
	order.RecipientName = input.RecipientName
	order.RecipientAddress = strings.TrimSpace(input.RecipientAddress)
	order.RecipientPhone = strings.TrimSpace(input.RecipientPhone)
	order.RecipientEmail = input.RecipientEmail

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
		return err
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


