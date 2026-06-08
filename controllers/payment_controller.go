package controllers

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"time"
	"teknik/services"
	"teknik/utils"

	"github.com/gofiber/fiber/v2"
	"github.com/xuri/excelize/v2"
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

// PaginatedPaymentsResponse adalah wrapper response paginasi untuk daftar pembayaran customer.
type PaginatedPaymentsResponse struct {
	Success    bool                              `json:"success"    example:"true"`
	Message    string                            `json:"message"    example:"Data tagihan dan pembayaran customer berhasil diambil"`
	Data       []services.CustomerPaymentSummary `json:"data"`
	Pagination PaginationMeta                    `json:"pagination"`
}

// PaginatedPaymentHistoryResponse adalah wrapper response paginasi untuk riwayat pembayaran.
type PaginatedPaymentHistoryResponse struct {
	Success    bool                         `json:"success"    example:"true"`
	Message    string                       `json:"message"    example:"Riwayat pembayaran berhasil diambil"`
	Data       []services.PaymentHistoryDTO `json:"data"`
	Pagination PaginationMeta               `json:"pagination"`
}


// ─────────────────────────────────────────────
// Get All Payments
// ─────────────────────────────────────────────

// GetAllPayments godoc
// @Summary      Ambil daftar pembayaran & tagihan customer
// @Description  Mengambil daftar customer beserta detail pesanan, total tagihan, dan histori pembayaran mereka. Dapat difilter berdasarkan nama customer, rentang tanggal order_date, status pembayaran, dan mendukung pagination (page & limit).
// @Tags         Payments
// @Param        name        query  string  false  "Nama Customer (opsional)"
// @Param        start_date  query  string  false  "Tanggal awal filter (YYYY-MM-DD)"
// @Param        end_date    query  string  false  "Tanggal akhir filter (YYYY-MM-DD)"
// @Param        status      query  string  false  "Status pembayaran (all, unpaid, partial, paid)"
// @Param        page        query  int     false  "Halaman aktif (default: 1)"
// @Param        limit       query  int     false  "Jumlah baris per halaman (default: 20)"
// @Produce      json
// @Success      200  {object}  controllers.PaginatedPaymentsResponse
// @Router       /payments [get]
// @Security     BearerAuth
func GetAllPayments(c *fiber.Ctx) error {
	name := c.Query("name")
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")
	status := c.Query("status")

	page := 1
	if p := c.Query("page"); p != "" {
		if val, err := strconv.Atoi(p); err == nil && val > 0 {
			page = val
		}
	}

	limit := 20
	if l := c.Query("limit"); l != "" {
		if val, err := strconv.Atoi(l); err == nil && val > 0 {
			limit = val
		}
	}

	results, totalRows, err := services.SearchCustomerPayments(name, startDate, endDate, status, page, limit)
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}

	totalPages := 0
	if totalRows > 0 {
		totalPages = int((totalRows + int64(limit) - 1) / int64(limit))
	}

	return c.Status(fiber.StatusOK).JSON(PaginatedPaymentsResponse{
		Success: true,
		Message: "Data tagihan dan pembayaran customer berhasil diambil",
		Data:    results,
		Pagination: PaginationMeta{
			Page:       page,
			Limit:      limit,
			TotalRows:  totalRows,
			TotalPages: totalPages,
		},
	})
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
// @Description  Mengambil detail ringkasan keuangan dan riwayat pembayaran lengkap dari satu customer tertentu berdasarkan namanya. Mendukung filter nomor PO/transaksi, status pembayaran (all, paid, partial, unpaid), dan rentang tanggal order_date.
// @Tags         Payments
// @Param        name        path      string  true   "Customer Name"
// @Param        po_no       query     string  false  "Cari nomor PO atau Transaksi"
// @Param        status      query     string  false  "Filter status (all, paid, partial, unpaid)"
// @Param        start_date  query     string  false  "Tanggal awal filter (YYYY-MM-DD)"
// @Param        end_date    query     string  false  "Tanggal akhir filter (YYYY-MM-DD)"
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
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")

	detail, err := services.GetCustomerPaymentDetail(name, poNo, status, startDate, endDate)
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
// @Description  Mengambil riwayat cicilan transaksi pembayaran dengan pencarian berdasarkan PO atau nomor transaksi, filter berdasarkan status pembayaran order terkait (all, paid, partial), rentang tanggal payments.payment_date, dan pagination (page & limit).
// @Tags         Payments
// @Produce      json
// @Param        search      query  string  false  "Cari nomor PO atau transaksi"
// @Param        status      query  string  false  "Filter status (all, paid, partial)"
// @Param        start_date  query  string  false  "Tanggal awal filter (YYYY-MM-DD)"
// @Param        end_date    query  string  false  "Tanggal akhir filter (YYYY-MM-DD)"
// @Param        page        query  int     false  "Halaman aktif (default: 1)"
// @Param        limit       query  int     false  "Jumlah baris per halaman (default: 20)"
// @Success      200  {object}  controllers.PaginatedPaymentHistoryResponse
// @Router       /payments/history [get]
// @Security     BearerAuth
func GetPaymentHistory(c *fiber.Ctx) error {
	search := c.Query("search")
	status := c.Query("status", "all")
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")

	page := 1
	if p := c.Query("page"); p != "" {
		if val, err := strconv.Atoi(p); err == nil && val > 0 {
			page = val
		}
	}

	limit := 20
	if l := c.Query("limit"); l != "" {
		if val, err := strconv.Atoi(l); err == nil && val > 0 {
			limit = val
		}
	}

	history, totalRows, err := services.GetPaymentHistory(search, status, startDate, endDate, page, limit)
	if err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, "Gagal mengambil riwayat pembayaran: "+err.Error())
	}

	totalPages := 0
	if totalRows > 0 {
		totalPages = int((totalRows + int64(limit) - 1) / int64(limit))
	}

	return c.Status(fiber.StatusOK).JSON(PaginatedPaymentHistoryResponse{
		Success: true,
		Message: "Riwayat pembayaran berhasil diambil",
		Data:    history,
		Pagination: PaginationMeta{
			Page:       page,
			Limit:      limit,
			TotalRows:  totalRows,
			TotalPages: totalPages,
		},
	})
}

// ─────────────────────────────────────────────
// Export Payment History to Excel
// ─────────────────────────────────────────────

// ExportPaymentHistory godoc
// @Summary      Export Riwayat Pembayaran ke Excel
// @Description  Mengunduh file Excel berisi riwayat alokasi cicilan pembayaran berdasarkan filter pencarian, status pembayaran order terkait, dan rentang tanggal payments.payment_date.
// @Tags         Payments
// @Produce      application/vnd.openxmlformats-officedocument.spreadsheetml.sheet
// @Param        search      query  string  false  "Cari nomor PO atau transaksi"
// @Param        status      query  string  false  "Filter status (all, paid, partial)"
// @Param        start_date  query  string  false  "Tanggal awal filter (YYYY-MM-DD)"
// @Param        end_date    query  string  false  "Tanggal akhir filter (YYYY-MM-DD)"
// @Success      200  {string}  string  "File Excel"
// @Failure      500  {object}  utils.Response
// @Router       /payments/history/export [get]
// @Security     BearerAuth
func ExportPaymentHistory(c *fiber.Ctx) error {
	search := c.Query("search")
	status := c.Query("status", "all")
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")

	history, _, err := services.GetPaymentHistory(search, status, startDate, endDate, 0, 0)
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}

	xf := excelize.NewFile()
	defer xf.Close()
	sh := "Riwayat Pembayaran"
	xf.SetSheetName("Sheet1", sh)

	st, err := newPaymentExcelStyles(xf)
	if err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}

	// ── Title ──
	xf.MergeCell(sh, "A1", "H1")
	xf.SetCellValue(sh, "A1", "LAPORAN RIWAYAT PEMBAYARAN")
	xf.SetCellStyle(sh, "A1", "H1", st.titleStyle)
	xf.SetRowHeight(sh, 1, 28)

	// ── Meta summary ──
	setPaymentMetaRow(xf, sh, 2, "Waktu Ekspor", time.Now().Local().Format("2006-01-02 15:04"), st.metaKey, st.metaVal)
	setPaymentMetaRow(xf, sh, 3, "Filter Pencarian", search, st.metaKey, st.metaVal)
	setPaymentMetaRow(xf, sh, 4, "Filter Status", status, st.metaKey, st.metaVal)

	// ── Header row ──
	headers := []string{"No", "Nomor Transaksi", "Nomor PO", "Nama Konsumen", "Admin Pembuat", "Tanggal Bayar", "Jumlah Bayar", "Status Pembayaran Order"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 6)
		xf.SetCellValue(sh, cell, h)
		xf.SetCellStyle(sh, cell, cell, st.headerStyle)
	}
	xf.SetRowHeight(sh, 6, 22)

	// ── Data rows ──
	var totalAmt float64
	for i, item := range history {
		row := 7 + i
		textSt, numSt := st.dataStyle, st.currency
		if i%2 == 1 {
			textSt, numSt = st.dataAlt, st.currencyAlt
		}
		xf.SetCellInt(sh, fmt.Sprintf("A%d", row), int64(i+1))
		xf.SetCellValue(sh, fmt.Sprintf("B%d", row), item.TransactionNo)
		xf.SetCellValue(sh, fmt.Sprintf("C%d", row), item.PoNo)
		xf.SetCellValue(sh, fmt.Sprintf("D%d", row), item.CustomerName)
		xf.SetCellValue(sh, fmt.Sprintf("E%d", row), item.AdminName)
		xf.SetCellValue(sh, fmt.Sprintf("F%d", row), item.CreatedAt)
		xf.SetCellFloat(sh, fmt.Sprintf("G%d", row), item.TotalAmount, 2, 64)
		xf.SetCellValue(sh, fmt.Sprintf("H%d", row), item.PaymentStatus)
		applyPaymentRowStyle(xf, sh, row, 8, textSt, numSt, 7)
		xf.SetRowHeight(sh, row, 18)
		totalAmt += item.TotalAmount
	}

	// ── Summary/total row ──
	sumRow := 7 + len(history)
	xf.MergeCell(sh, fmt.Sprintf("A%d", sumRow), fmt.Sprintf("F%d", sumRow))
	xf.SetCellValue(sh, fmt.Sprintf("A%d", sumRow), "TOTAL TERBAYAR")
	xf.SetCellStyle(sh, fmt.Sprintf("A%d", sumRow), fmt.Sprintf("F%d", sumRow), st.sumLabel)
	xf.SetCellFloat(sh, fmt.Sprintf("G%d", sumRow), totalAmt, 2, 64)
	xf.SetCellStyle(sh, fmt.Sprintf("G%d", sumRow), fmt.Sprintf("G%d", sumRow), st.sumValue)
	xf.SetCellStyle(sh, fmt.Sprintf("H%d", sumRow), fmt.Sprintf("H%d", sumRow), st.sumLabel)
	xf.SetRowHeight(sh, sumRow, 20)

	// ── Column widths ──
	xf.SetColWidth(sh, "A", "A", 5)
	xf.SetColWidth(sh, "B", "B", 18)
	xf.SetColWidth(sh, "C", "C", 16)
	xf.SetColWidth(sh, "D", "D", 30)
	xf.SetColWidth(sh, "E", "E", 18)
	xf.SetColWidth(sh, "F", "F", 18)
	xf.SetColWidth(sh, "G", "G", 20)
	xf.SetColWidth(sh, "H", "H", 22)

	buf := new(bytes.Buffer)
	if err := xf.Write(buf); err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}

	c.Set(fiber.HeaderContentType, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Set(fiber.HeaderContentDisposition, fmt.Sprintf("attachment; filename=\"riwayat-pembayaran-%s.xlsx\"", time.Now().Format("2006-01-02-150405")))
	return c.Send(buf.Bytes())
}

// ─────────────────────────────────────────────
// EXCEL STYLES HELPERS FOR PAYMENTS
// ─────────────────────────────────────────────

type paymentExcelStyles struct {
	titleStyle  int
	metaKey     int
	metaVal     int
	headerStyle int
	dataStyle   int
	dataAlt     int
	currency    int
	currencyAlt int
	sumLabel    int
	sumValue    int
}

func newPaymentExcelStyles(f *excelize.File) (paymentExcelStyles, error) {
	var s paymentExcelStyles
	var err error

	border := []excelize.Border{
		{Type: "left", Color: "BDBDBD", Style: 1},
		{Type: "right", Color: "BDBDBD", Style: 1},
		{Type: "top", Color: "BDBDBD", Style: 1},
		{Type: "bottom", Color: "BDBDBD", Style: 1},
	}

	s.titleStyle, err = f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 16, Color: "1A237E", Family: "Calibri"},
		Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "center"},
	})
	if err != nil {
		return s, err
	}

	s.metaKey, err = f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 10, Color: "424242", Family: "Calibri"},
		Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "center"},
	})
	if err != nil {
		return s, err
	}

	s.metaVal, err = f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Size: 10, Color: "212121", Family: "Calibri"},
		Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "center"},
	})
	if err != nil {
		return s, err
	}

	s.headerStyle, err = f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 10, Color: "FFFFFF", Family: "Calibri"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"1A237E"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
		Border:    border,
	})
	if err != nil {
		return s, err
	}

	s.dataStyle, err = f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Size: 10, Color: "212121", Family: "Calibri"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"FFFFFF"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "center"},
		Border:    border,
	})
	if err != nil {
		return s, err
	}

	s.dataAlt, err = f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Size: 10, Color: "212121", Family: "Calibri"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"E8EAF6"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "center"},
		Border:    border,
	})
	if err != nil {
		return s, err
	}

	s.currency, err = f.NewStyle(&excelize.Style{
		Font:         &excelize.Font{Size: 10, Color: "212121", Family: "Calibri"},
		Fill:         excelize.Fill{Type: "pattern", Color: []string{"FFFFFF"}, Pattern: 1},
		Alignment:    &excelize.Alignment{Horizontal: "right", Vertical: "center"},
		Border:       border,
		CustomNumFmt: strPtr(`"Rp "#,##0.00`),
	})
	if err != nil {
		return s, err
	}

	s.currencyAlt, err = f.NewStyle(&excelize.Style{
		Font:         &excelize.Font{Size: 10, Color: "212121", Family: "Calibri"},
		Fill:         excelize.Fill{Type: "pattern", Color: []string{"E8EAF6"}, Pattern: 1},
		Alignment:    &excelize.Alignment{Horizontal: "right", Vertical: "center"},
		Border:       border,
		CustomNumFmt: strPtr(`"Rp "#,##0.00`),
	})
	if err != nil {
		return s, err
	}

	s.sumLabel, err = f.NewStyle(&excelize.Style{
		Font:   &excelize.Font{Bold: true, Size: 10, Color: "FFFFFF", Family: "Calibri"},
		Fill:   excelize.Fill{Type: "pattern", Color: []string{"283593"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "right", Vertical: "center"},
		Border: border,
	})
	if err != nil {
		return s, err
	}

	s.sumValue, err = f.NewStyle(&excelize.Style{
		Font:         &excelize.Font{Bold: true, Size: 10, Color: "FFFFFF", Family: "Calibri"},
		Fill:         excelize.Fill{Type: "pattern", Color: []string{"283593"}, Pattern: 1},
		Alignment:    &excelize.Alignment{Horizontal: "right", Vertical: "center"},
		Border:       border,
		CustomNumFmt: strPtr(`"Rp "#,##0.00`),
	})
	if err != nil {
		return s, err
	}

	return s, nil
}

func setPaymentMetaRow(f *excelize.File, sh string, row int, label, value string, labelStyle, valStyle int) {
	cell := fmt.Sprintf("A%d", row)
	f.SetCellValue(sh, cell, label)
	f.SetCellStyle(sh, cell, cell, labelStyle)
	v := fmt.Sprintf("B%d", row)
	f.SetCellValue(sh, v, value)
	f.SetCellStyle(sh, v, v, valStyle)
}

func applyPaymentRowStyle(f *excelize.File, sh string, row, numCols int, textSt, numSt int, numCol int) {
	for col := 1; col <= numCols; col++ {
		cellRef, _ := excelize.CoordinatesToCellName(col, row)
		if col == numCol {
			f.SetCellStyle(sh, cellRef, cellRef, numSt)
		} else {
			f.SetCellStyle(sh, cellRef, cellRef, textSt)
		}
	}
}


