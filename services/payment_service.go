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

// UpdatePaymentTotal memperbarui payment_total pembayaran yang ada dan mengalokasikan ulang secara atomik.
func UpdatePaymentTotal(paymentIDStr string, newTotal float64, userID uuid.UUID) (models.Payment, error) {
	paymentID, err := uuid.Parse(paymentIDStr)
	if err != nil {
		return models.Payment{}, fmt.Errorf("ID pembayaran tidak valid")
	}

	if newTotal <= 0 {
		return models.Payment{}, fmt.Errorf("total pembayaran baru harus lebih dari 0")
	}

	// 1. Mulai transaksi database
	tx := config.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 2. Ambil data payment beserta details lama
	var payment models.Payment
	if err := tx.Preload("Details").First(&payment, "id = ?", paymentID).Error; err != nil {
		tx.Rollback()
		return models.Payment{}, fmt.Errorf("pembayaran tidak ditemukan: %w", err)
	}

	// Buat salinan data payment lama untuk audit log
	oldPaymentData := payment

	// 3. REVERT: Kembalikan alokasi lama ke masing-masing Invoice
	invoiceAllocations := make(map[uuid.UUID]float64)
	var invoiceIDs []uuid.UUID
	for _, detail := range payment.Details {
		invoiceAllocations[detail.InvoiceID] += detail.AllocatedAmount
		invoiceIDs = append(invoiceIDs, detail.InvoiceID)
	}

	if len(invoiceIDs) == 0 {
		tx.Rollback()
		return models.Payment{}, fmt.Errorf("pembayaran tidak memiliki detail alokasi")
	}

	// Ambil data invoice yang terdampak
	var invoices []models.Invoice
	if err := tx.Where("id IN ?", invoiceIDs).
		Order("created_at ASC").
		Find(&invoices).Error; err != nil {
		tx.Rollback()
		return models.Payment{}, fmt.Errorf("gagal mengambil data invoice terkait: %w", err)
	}

	// Buat map invoice untuk mempermudah update
	invoiceMap := make(map[uuid.UUID]*models.Invoice)
	for i := range invoices {
		invoiceMap[invoices[i].ID] = &invoices[i]
	}

	// Kembalikan saldo dan status masing-masing invoice
	for invID, oldAlloc := range invoiceAllocations {
		inv, exists := invoiceMap[invID]
		if !exists {
			tx.Rollback()
			return models.Payment{}, fmt.Errorf("invoice %s tidak ditemukan dalam database", invID)
		}

		inv.RemainingBalance = roundTwo(inv.RemainingBalance + oldAlloc)
		if inv.RemainingBalance >= inv.TotalAmount {
			inv.PaymentStatus = models.PaymentStatusUnpaid
			inv.RemainingBalance = inv.TotalAmount
		} else if inv.RemainingBalance > 0 {
			inv.PaymentStatus = models.PaymentStatusPartial
		} else {
			inv.PaymentStatus = models.PaymentStatusPaid
		}

		if err := tx.Save(inv).Error; err != nil {
			tx.Rollback()
			return models.Payment{}, fmt.Errorf("gagal mengembalikan saldo invoice %s: %w", inv.ID, err)
		}
	}

	// 4. Validasi batas total tagihan yang telah direvert
	var totalTagihanReverted float64
	for _, inv := range invoices {
		totalTagihanReverted += inv.RemainingBalance
	}

	if newTotal > totalTagihanReverted {
		tx.Rollback()
		return models.Payment{}, fmt.Errorf("jumlah pembayaran baru (%.2f) melebihi total sisa tagihan yang tersedia (%.2f)", newTotal, totalTagihanReverted)
	}

	// 5. Hapus detail pembayaran lama
	if err := tx.Where("payment_id = ?", payment.ID).Delete(&models.PaymentDetail{}).Error; err != nil {
		tx.Rollback()
		return models.Payment{}, fmt.Errorf("gagal menghapus detail alokasi lama: %w", err)
	}

	// 6. REALLOCATE: Alokasikan newTotal secara kronologis ke invoice-invoice tersebut
	var newDetails []models.PaymentDetail
	sisaDana := newTotal

	for i := range invoices {
		inv := &invoices[i]
		if sisaDana <= 0 {
			break
		}

		alokasi := sisaDana
		if alokasi > inv.RemainingBalance {
			alokasi = inv.RemainingBalance
		}

		// Update invoice dengan alokasi baru
		inv.RemainingBalance = roundTwo(inv.RemainingBalance - alokasi)
		if inv.RemainingBalance <= 0 {
			inv.PaymentStatus = models.PaymentStatusPaid
			inv.RemainingBalance = 0
		} else {
			inv.PaymentStatus = models.PaymentStatusPartial
		}

		if err := tx.Save(inv).Error; err != nil {
			tx.Rollback()
			return models.Payment{}, fmt.Errorf("gagal memperbarui alokasi saldo invoice %s: %w", inv.ID, err)
		}

		// Buat detail alokasi baru
		newDetail := models.PaymentDetail{
			ID:              uuid.New(),
			PaymentID:       payment.ID,
			InvoiceID:       inv.ID,
			AllocatedAmount: alokasi,
		}

		if err := tx.Create(&newDetail).Error; err != nil {
			tx.Rollback()
			return models.Payment{}, fmt.Errorf("gagal menyimpan detail alokasi baru: %w", err)
		}

		newDetails = append(newDetails, newDetail)
		sisaDana = roundTwo(sisaDana - alokasi)
	}

	// 7. Update Payment utama
	payment.PaymentTotal = newTotal
	payment.Details = newDetails
	if err := tx.Save(&payment).Error; err != nil {
		tx.Rollback()
		return models.Payment{}, fmt.Errorf("gagal memperbarui pembayaran utama: %w", err)
	}

	// Commit transaksi
	if err := tx.Commit().Error; err != nil {
		return models.Payment{}, fmt.Errorf("gagal melakukan commit transaksi: %w", err)
	}

	// 8. Catat Audit Log UPDATE
	CreateAuditLog(userID, payment.ID, models.AuditActionUpdate, "payments", oldPaymentData, payment)

	return payment, nil
}

