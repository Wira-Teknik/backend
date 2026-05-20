package services

import (
	"fmt"
	"time"

	"teknik/config"
	"teknik/models"
	"teknik/utils"

	"github.com/google/uuid"
)

// ─────────────────────────────────────────────
// DTOs
// ─────────────────────────────────────────────

type CreatePaymentInput struct {
	PaymentDate  string   `json:"payment_date"` // format: "2006-01-02"
	PaymentTotal float64  `json:"payment_total"`
	OrderIDs     []string `json:"order_ids"`
}

// ─────────────────────────────────────────────
// Get All Payments
// ─────────────────────────────────────────────

type CustomerPaymentSummary struct {
	CustomerName string         `json:"customer_name"`
	Orders       []models.Order `json:"orders"`
	TotalTagihan float64        `json:"total_tagihan"`
}

// SearchCustomerPayments mencari customer berdasarkan nama (dari data pesanan) dan menghitung tagihannya.
func SearchCustomerPayments(name string) ([]CustomerPaymentSummary, error) {
	var orders []models.Order

	query := config.DB.Preload("Items").
		Preload("Shipments").
		Preload("Shipments.Items").
		Preload("Shipments.Invoice").
		Order("created_at DESC")

	if name != "" {
		query = query.Where("recipient_name ILIKE ?", "%"+name+"%")
	}

	if err := query.Find(&orders).Error; err != nil {
		return nil, err
	}

	// Group by recipient_name
	customerMap := make(map[string]*CustomerPaymentSummary)

	for i := range orders {
		computeOrderPaymentInfo(&orders[i])

		custName := orders[i].RecipientName
		if _, exists := customerMap[custName]; !exists {
			customerMap[custName] = &CustomerPaymentSummary{
				CustomerName: custName,
				Orders:       []models.Order{},
				TotalTagihan: 0,
			}
		}

		customerMap[custName].Orders = append(customerMap[custName].Orders, orders[i])
		customerMap[custName].TotalTagihan += orders[i].RemainingBalance
	}

	var results []CustomerPaymentSummary
	for _, summary := range customerMap {
		summary.TotalTagihan = roundTwo(summary.TotalTagihan)
		results = append(results, *summary)
	}

	return results, nil
}

func GetAllPayments() ([]models.Payment, error) {
	var payments []models.Payment
	err := config.DB.Preload("Details").
		Order("created_at DESC").
		Find(&payments).Error
	return payments, err
}

// ─────────────────────────────────────────────
// Get Payment By ID
// ─────────────────────────────────────────────

func GetPaymentByID(id string) (models.Payment, error) {
	var payment models.Payment
	err := config.DB.Preload("Details").
		First(&payment, "id = ?", id).Error
	return payment, err
}

// ─────────────────────────────────────────────
// Create Payment (Lunas / Cicilan / Kolektif)
// ─────────────────────────────────────────────
// Alur:
// 1. Validasi setiap detail: invoice harus ada dan belum lunas
// 2. allocated_amount <= remaining_balance
// 3. Buat Payment + PaymentDetails dalam transaksi
// 4. Update remaining_balance & payment_status per invoice
// 5. payment_total = sum(allocated_amount)

func CreatePayment(input CreatePaymentInput, userID uuid.UUID) (models.Payment, error) {
	if len(input.OrderIDs) == 0 {
		return models.Payment{}, fmt.Errorf("pembayaran harus memiliki minimal 1 order ID")
	}
	if input.PaymentTotal <= 0 {
		return models.Payment{}, fmt.Errorf("total pembayaran harus lebih dari 0")
	}

	paymentDate, err := time.Parse("2006-01-02", input.PaymentDate)
	if err != nil {
		return models.Payment{}, fmt.Errorf("format tanggal tidak valid (gunakan YYYY-MM-DD)")
	}

	// 1. Ambil semua invoice terkait order_ids yang belum lunas
	var invoices []models.Invoice

	// Join melalui shipment. Order -> Shipment -> Invoice.
	err = config.DB.Joins("JOIN shipments ON shipments.id = invoices.shipment_id").
		Where("shipments.order_id IN ? AND invoices.payment_status != ?", input.OrderIDs, models.PaymentStatusPaid).
		Order("invoices.created_at ASC"). // urutkan yang paling lama dahulu
		Find(&invoices).Error

	if err != nil {
		return models.Payment{}, fmt.Errorf("gagal mengambil tagihan: %w", err)
	}

	if len(invoices) == 0 {
		return models.Payment{}, fmt.Errorf("tidak ada tagihan yang belum lunas untuk order yang dipilih")
	}

	// 2. Validasi total payment <= total remaining balance
	var totalTagihan float64
	for _, inv := range invoices {
		totalTagihan += inv.RemainingBalance
	}

	if input.PaymentTotal > totalTagihan {
		return models.Payment{}, fmt.Errorf("jumlah pembayaran (%.2f) melebihi total sisa tagihan (%.2f)", input.PaymentTotal, totalTagihan)
	}

	// 3. Mulai alokasi dana secara otomatis
	paymentID := uuid.New()
	var details []models.PaymentDetail

	type invoiceAlloc struct {
		invoice         models.Invoice
		allocatedAmount float64
	}
	var allocations []invoiceAlloc

	sisaPembayaran := input.PaymentTotal

	for _, inv := range invoices {
		if sisaPembayaran <= 0 {
			break
		}

		alokasi := sisaPembayaran
		if alokasi > inv.RemainingBalance {
			alokasi = inv.RemainingBalance
		}

		details = append(details, models.PaymentDetail{
			ID:              uuid.New(),
			PaymentID:       paymentID,
			InvoiceID:       inv.ID,
			AllocatedAmount: alokasi,
		})

		allocations = append(allocations, invoiceAlloc{
			invoice:         inv,
			allocatedAmount: alokasi,
		})

		sisaPembayaran -= alokasi
		sisaPembayaran = roundTwo(sisaPembayaran)
	}

	// 4. Mulai transaksi database
	tx := config.DB.Begin()

	payment := models.Payment{
		ID:           paymentID,
		PaymentTotal: input.PaymentTotal,
		PaymentDate:  utils.JSONDate(paymentDate),
		Details:      details,
	}

	if err := tx.Create(&payment).Error; err != nil {
		tx.Rollback()
		return models.Payment{}, fmt.Errorf("gagal membuat pembayaran: %w", err)
	}

	// Update setiap invoice
	for _, alloc := range allocations {
		newBalance := roundTwo(alloc.invoice.RemainingBalance - alloc.allocatedAmount)

		var newStatus models.PaymentStatus
		if newBalance <= 0 {
			newStatus = models.PaymentStatusPaid
			newBalance = 0
		} else {
			newStatus = models.PaymentStatusPartial
		}

		if err := tx.Model(&models.Invoice{}).
			Where("id = ?", alloc.invoice.ID).
			Updates(map[string]interface{}{
				"remaining_balance": newBalance,
				"payment_status":    newStatus,
			}).Error; err != nil {
			tx.Rollback()
			return models.Payment{}, fmt.Errorf("gagal memperbarui invoice: %w", err)
		}
	}

	if err := tx.Commit().Error; err != nil {
		return models.Payment{}, fmt.Errorf("gagal menyimpan data pembayaran: %w", err)
	}

	CreateAuditLog(userID, paymentID, models.AuditActionCreate, "payments", nil, payment)

	return payment, nil
}
