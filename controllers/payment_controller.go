package controllers

import (
	"encoding/json"
	"errors"
	"net/url"
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

// UpdatePaymentTotalRequest adalah request body untuk memperbarui total pembayaran.
type UpdatePaymentTotalRequest struct {
	PaymentTotal float64 `json:"payment_total" example:"7500000"`
}

// ─────────────────────────────────────────────
// Get All Payments
// ─────────────────────────────────────────────

// GetAllPayments godoc
// @Summary      Ambil daftar pembayaran & tagihan customer
// @Description  Mengambil daftar customer beserta detail pesanan, total tagihan, dan histori pembayaran mereka. Dapat difilter berdasarkan nama customer.
// @Tags         Payments
// @Param        name query string false "Nama Customer (opsional)"
// @Produce      json
// @Success      200  {object}  utils.Response{data=[]services.CustomerPaymentSummary}
// @Router       /payments [get]
// @Security     BearerAuth
func GetAllPayments(c *fiber.Ctx) error {
	name := c.Query("name")
	results, err := services.SearchCustomerPayments(name)
	if err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, "Gagal mengambil data tagihan dan pembayaran customer")
	}
	return utils.JSONSuccess(c, "Data tagihan dan pembayaran customer berhasil diambil", results)
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
		if errors.Is(err, services.ErrPaymentInvalidUUID) {
			return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
		}
		if errors.Is(err, services.ErrPaymentNotFound) {
			return utils.JSONError(c, fiber.StatusNotFound, err.Error())
		}
		return utils.JSONError(c, fiber.StatusInternalServerError, "Gagal mengambil detail pembayaran")
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
// @Param        details       formData  string  true  "JSON string of array of details e.g. [ { ''order_id'': ''uuid'' } ] (NOTE: Ganti '' dengan tanda kutip dua)"
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

// UpdatePaymentTotal godoc
// @Summary      Edit total pembayaran
// @Description  Memperbarui jumlah total pembayaran (payment_total) dan secara otomatis mengalokasikan ulang sisa saldo ke invoice yang bersangkutan secara kronologis.
// @Tags         Payments
// @Accept       json
// @Produce      json
// @Param        id            path      string                     true  "Payment ID"
// @Param        request       body      UpdatePaymentTotalRequest  true  "Request Body"
// @Success      200           {object}  utils.Response{data=models.Payment}
// @Failure      400           {object}  utils.Response
// @Failure      404           {object}  utils.Response
// @Router       /payments/{id} [put]
// @Security     BearerAuth
func UpdatePaymentTotal(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return utils.JSONError(c, fiber.StatusBadRequest, "ID pembayaran tidak boleh kosong")
	}

	var req UpdatePaymentTotalRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "Format request tidak valid")
	}

	if req.PaymentTotal <= 0 {
		return utils.JSONError(c, fiber.StatusBadRequest, "Total pembayaran harus berupa angka valid dan lebih dari 0")
	}

	userID, err := services.ParseUserID(c.Locals("userID").(string))
	if err != nil {
		return utils.JSONError(c, fiber.StatusUnauthorized, err.Error())
	}

	payment, err := services.UpdatePaymentTotal(id, req.PaymentTotal, userID)
	if err != nil {
		if errors.Is(err, services.ErrPaymentInvalidUUID) {
			return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
		}
		if errors.Is(err, services.ErrPaymentNotFound) {
			return utils.JSONError(c, fiber.StatusNotFound, err.Error())
		}
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}

	return utils.JSONSuccess(c, "Total pembayaran berhasil diperbarui", payment)
}

// GetCustomerPaymentDetail godoc
// @Summary      Ambil detail pembayaran customer dengan filter
// @Description  Mengambil detail ringkasan keuangan dan riwayat pembayaran lengkap dari satu customer tertentu berdasarkan namanya. Mendukung filter nomor PO/transaksi dan status pembayaran (all, paid, partial, unpaid).
// @Tags         Payments
// @Param        name    path      string  true   "Customer Name"
// @Param        po_no   query     string  false  "Cari nomor PO atau Transaksi"
// @Param        status  query     string  false  "Filter status (all, paid, partial, unpaid)"
// @Produce      json
// @Success      200     {object}  utils.Response{data=services.CustomerPaymentDetailResponse}
// @Failure      404     {object}  utils.Response
// @Router       /payments/customer/{name} [get]
// @Security     BearerAuth
func GetCustomerPaymentDetail(c *fiber.Ctx) error {
	name := c.Params("name")
	if name == "" {
		return utils.JSONError(c, fiber.StatusBadRequest, "Nama customer tidak boleh kosong")
	}

	// Lakukan URL Path Unescape agar parameter dengan spasi (seperti %20) kembali ke teks aslinya
	unescapedName, errEsc := url.PathUnescape(name)
	if errEsc == nil {
		name = unescapedName
	}

	poNo := c.Query("po_no")
	status := c.Query("status")

	detail, err := services.GetCustomerPaymentDetail(name, poNo, status)
	if err != nil {
		return utils.JSONError(c, fiber.StatusNotFound, err.Error())
	}

	return utils.JSONSuccess(c, "Detail pembayaran customer berhasil diambil", detail)
}

// ─────────────────────────────────────────────
// Payment History Report
// ─────────────────────────────────────────────

// GetPaymentHistory godoc
// @Summary      Ambil Laporan Riwayat Transaksi Pembayaran
// @Description  Mengambil riwayat cicilan transaksi pembayaran dengan pencarian berdasarkan PO atau nomor transaksi dan filter berdasarkan status pembayaran order terkait (all, paid, partial)
// @Tags         Payments
// @Produce      json
// @Param        search query string false "Cari nomor PO atau transaksi"
// @Param        status query string false "Filter status (all, paid, partial)"
// @Success      200  {object}  utils.Response{data=[]services.PaymentHistoryDTO}
// @Router       /payments/history [get]
// @Security     BearerAuth
func GetPaymentHistory(c *fiber.Ctx) error {
	search := c.Query("search")
	status := c.Query("status", "all")

	history, err := services.GetPaymentHistory(search, status)
	if err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, "Gagal mengambil riwayat pembayaran: "+err.Error())
	}
	return utils.JSONSuccess(c, "Riwayat pembayaran berhasil diambil", history)
}

