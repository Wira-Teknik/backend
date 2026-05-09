package controllers

import (
	"teknik/services"
	"teknik/utils"

	"github.com/gofiber/fiber/v2"
)

// ─────────────────────────────────────────────
// Request DTOs (exported for Swagger)
// ─────────────────────────────────────────────

// CreatePaymentRequest adalah body untuk membuat pembayaran.
type CreatePaymentRequest struct {
	PaymentDate string                        `json:"payment_date" example:"2026-05-15"`
	Details     []PaymentDetailRequestPayload `json:"details"`
}

// PaymentDetailRequestPayload adalah detail alokasi pembayaran ke invoice.
type PaymentDetailRequestPayload struct {
	InvoiceID       string  `json:"invoice_id"       example:"550e8400-e29b-41d4-a716-446655440002"`
	AllocatedAmount float64 `json:"allocated_amount"  example:"5000000"`
}

// ─────────────────────────────────────────────
// Get All Payments
// ─────────────────────────────────────────────

// GetAllPayments godoc
// @Summary      Ambil semua pembayaran
// @Description  Mengambil daftar semua pembayaran beserta detail alokasinya
// @Tags         Payments
// @Produce      json
// @Success      200  {object}  utils.Response{data=[]models.Payment}
// @Router       /payments [get]
// @Security     BearerAuth
func GetAllPayments(c *fiber.Ctx) error {
	payments, err := services.GetAllPayments()
	if err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, "Gagal mengambil data pembayaran")
	}
	return utils.JSONSuccess(c, "Data pembayaran berhasil diambil", payments)
}

// ─────────────────────────────────────────────
// Get Payment by ID
// ─────────────────────────────────────────────

// GetPayment godoc
// @Summary      Ambil detail pembayaran
// @Description  Mengambil detail pembayaran termasuk detail alokasi ke invoice
// @Tags         Payments
// @Param        id   path      string  true  "Payment ID"
// @Produce      json
// @Success      200  {object}  utils.Response{data=models.Payment}
// @Failure      404  {object}  utils.Response
// @Router       /payments/{id} [get]
// @Security     BearerAuth
func GetPayment(c *fiber.Ctx) error {
	id := c.Params("id")
	payment, err := services.GetPaymentByID(id)
	if err != nil {
		return utils.JSONError(c, fiber.StatusNotFound, "Pembayaran tidak ditemukan")
	}
	return utils.JSONSuccess(c, "Detail pembayaran berhasil diambil", payment)
}

// ─────────────────────────────────────────────
// Create Payment
// ─────────────────────────────────────────────

// CreatePayment godoc
// @Summary      Buat pembayaran baru
// @Description  Membuat pembayaran baru dengan alokasi ke satu atau lebih invoice. Mendukung 3 skenario: lunas (allocated = total), cicilan (allocated < total), dan kolektif (satu payment untuk beberapa invoice). payment_total dihitung otomatis dari sum allocated_amount.
// @Tags         Payments
// @Accept       json
// @Produce      json
// @Param        body  body      CreatePaymentRequest  true  "Data pembayaran"
// @Success      201   {object}  utils.Response{data=models.Payment}
// @Failure      400   {object}  utils.Response
// @Router       /payments [post]
// @Security     BearerAuth
func CreatePayment(c *fiber.Ctx) error {
	var req CreatePaymentRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "Format request tidak valid")
	}

	userID, err := services.ParseUserID(c.Locals("userID").(string))
	if err != nil {
		return utils.JSONError(c, fiber.StatusUnauthorized, err.Error())
	}

	var details []services.PaymentDetailInput
	for _, d := range req.Details {
		details = append(details, services.PaymentDetailInput{
			InvoiceID:       d.InvoiceID,
			AllocatedAmount: d.AllocatedAmount,
		})
	}

	payment, err := services.CreatePayment(services.CreatePaymentInput{
		PaymentDate: req.PaymentDate,
		Details:     details,
	}, userID)
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}

	return utils.JSONCreated(c, "Pembayaran berhasil dicatat", payment)
}
