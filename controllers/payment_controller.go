package controllers

import (
	"encoding/json"
	"strconv"
	"teknik/services"
	"teknik/utils"

	"github.com/gofiber/fiber/v2"
)

// ─────────────────────────────────────────────
// Request DTOs (exported for Swagger)
// ─────────────────────────────────────────────

// PaymentDetailRequest berisi informasi order yang akan dibayar.
type PaymentDetailRequest struct {
	OrderID string `json:"order_id" example:"550e8400-e29b-41d4-a716-446655440001"`
}

// CreatePaymentRequest adalah body untuk membuat pembayaran.
type CreatePaymentRequest struct {
	PaymentDate  string                 `json:"payment_date" example:"2026-05-15"`
	PaymentTotal float64                `json:"payment_total" example:"10000000"`
	Details      []PaymentDetailRequest `json:"details"`
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
// Search Customer Payments
// ─────────────────────────────────────────────

// SearchCustomerPayments godoc
// @Summary      Cari pembayaran berdasarkan nama customer
// @Description  Mencari customer dan mengembalikan daftar pesanan serta total sisa tagihan yang harus dibayar.
// @Tags         Payments
// @Param        name query string false "Nama Customer"
// @Produce      json
// @Success      200  {object}  utils.Response{data=[]services.CustomerPaymentSummary}
// @Router       /payments/search-customer [get]
// @Security     BearerAuth
func SearchCustomerPayments(c *fiber.Ctx) error {
	name := c.Query("name")
	results, err := services.SearchCustomerPayments(name)
	if err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, "Gagal mencari data tagihan customer")
	}
	return utils.JSONSuccess(c, "Data tagihan customer berhasil diambil", results)
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
// @Description  Membuat pembayaran baru dengan alokasi otomatis berdasarkan daftar Order ID dari tagihan yang terlama. File bukti bayar dapat diunggah secara terpisah melalui API Attachments.
// @Tags         Payments
// @Accept       multipart/form-data
// @Produce      json
// @Param        payment_date  formData  string  true  "Payment Date (YYYY-MM-DD)"
// @Param        payment_total formData  number  true  "Total transfer amount (untuk auto-allocation via order_id)"
// @Param        details       formData  string  true  "JSON string of array of details e.g. [{'order_id':'...'}]"
// @Param        bukti_bayar   formData  file    true  "File Bukti Pembayaran"
// @Success      201   {object}  utils.Response{data=models.Payment}
// @Failure      400   {object}  utils.Response
// @Router       /payments [post]
// @Security     BearerAuth
func CreatePayment(c *fiber.Ctx) error {
	paymentDate := c.FormValue("payment_date")
	detailsJSON := c.FormValue("details")
	paymentTotalStr := c.FormValue("payment_total")

	var detailsPayload []PaymentDetailRequest
	if detailsJSON != "" {
		if err := json.Unmarshal([]byte(detailsJSON), &detailsPayload); err != nil {
			return utils.JSONError(c, fiber.StatusBadRequest, "Format details tidak valid. Harus berupa array JSON.")
		}
	} else {
		return utils.JSONError(c, fiber.StatusBadRequest, "Details pembayaran tidak boleh kosong")
	}

	paymentTotal, errTotal := strconv.ParseFloat(paymentTotalStr, 64)
	if errTotal != nil || paymentTotal <= 0 {
		return utils.JSONError(c, fiber.StatusBadRequest, "Total pembayaran harus berupa angka valid dan lebih dari 0")
	}

	buktiBayar, errBukti := c.FormFile("bukti_bayar")
	if errBukti != nil || buktiBayar == nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "File Bukti Pembayaran wajib dilampirkan")
	}

	userID, err := services.ParseUserID(c.Locals("userID").(string))
	if err != nil {
		return utils.JSONError(c, fiber.StatusUnauthorized, err.Error())
	}

	var orderIDs []string
	for _, detail := range detailsPayload {
		if detail.OrderID != "" {
			orderIDs = append(orderIDs, detail.OrderID)
		}
	}

	payment, err := services.CreatePayment(services.CreatePaymentInput{
		PaymentDate:  paymentDate,
		PaymentTotal: paymentTotal,
		OrderIDs:     orderIDs,
	}, userID)

	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}

	// Upload file bukti_bayar
	services.UploadAttachment(buktiBayar, payment.ID.String(), "payment_proof", userID)

	return utils.JSONCreated(c, "Pembayaran berhasil dicatat", payment)
}
