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

type PaymentDetailInput struct {
	InvoiceID       string  `json:"invoice_id"`
	AllocatedAmount float64 `json:"allocated_amount"`
}

type CreatePaymentInput struct {
	PaymentDate string               `json:"payment_date"` // format: "2006-01-02"
	Details     []PaymentDetailInput `json:"details"`
}

// ─────────────────────────────────────────────
// Get All Payments
// ─────────────────────────────────────────────

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
	if len(input.Details) == 0 {
		return models.Payment{}, fmt.Errorf("pembayaran harus memiliki minimal 1 detail alokasi")
	}

	paymentDate, err := time.Parse("2006-01-02", input.PaymentDate)
	if err != nil {
		return models.Payment{}, fmt.Errorf("format tanggal tidak valid (gunakan YYYY-MM-DD)")
	}

	paymentID := uuid.New()
	var details []models.PaymentDetail
	var paymentTotal float64

	// Pre-validasi semua invoice
	type invoiceAlloc struct {
		invoice         models.Invoice
		allocatedAmount float64
	}
	var allocations []invoiceAlloc

	for _, d := range input.Details {
		invoiceID, err := uuid.Parse(d.InvoiceID)
		if err != nil {
			return models.Payment{}, fmt.Errorf("invoice ID tidak valid: %s", d.InvoiceID)
		}

		if d.AllocatedAmount <= 0 {
			return models.Payment{}, fmt.Errorf("jumlah alokasi harus lebih dari 0")
		}

		var invoice models.Invoice
		if err := config.DB.First(&invoice, "id = ?", invoiceID).Error; err != nil {
			return models.Payment{}, fmt.Errorf("invoice tidak ditemukan: %s", d.InvoiceID)
		}

		if invoice.PaymentStatus == models.PaymentStatusPaid {
			return models.Payment{}, fmt.Errorf("invoice %s sudah lunas", invoice.InvoiceNo)
		}

		if d.AllocatedAmount > invoice.RemainingBalance {
			return models.Payment{}, fmt.Errorf(
				"jumlah alokasi (%.2f) melebihi sisa tagihan (%.2f) untuk invoice %s",
				d.AllocatedAmount, invoice.RemainingBalance, invoice.InvoiceNo,
			)
		}

		details = append(details, models.PaymentDetail{
			ID:              uuid.New(),
			PaymentID:       paymentID,
			InvoiceID:       invoiceID,
			AllocatedAmount: d.AllocatedAmount,
		})

		allocations = append(allocations, invoiceAlloc{
			invoice:         invoice,
			allocatedAmount: d.AllocatedAmount,
		})

		paymentTotal += d.AllocatedAmount
	}

	paymentTotal = roundTwo(paymentTotal)

	// Mulai transaksi database
	tx := config.DB.Begin()

	// 1. Buat payment
	payment := models.Payment{
		ID:           paymentID,
		PaymentTotal: paymentTotal,
		PaymentDate:  utils.JSONDate(paymentDate),
		Details:      details,
	}

	if err := tx.Create(&payment).Error; err != nil {
		tx.Rollback()
		return models.Payment{}, fmt.Errorf("gagal membuat pembayaran: %w", err)
	}

	// 2. Update setiap invoice
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

	// Commit transaksi
	if err := tx.Commit().Error; err != nil {
		return models.Payment{}, fmt.Errorf("gagal menyimpan data pembayaran: %w", err)
	}

	// Audit log
	CreateAuditLog(userID, paymentID, models.AuditActionCreate, "payments", nil, payment)

	return payment, nil
}
