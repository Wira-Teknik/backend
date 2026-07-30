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
	"github.com/go-pdf/fpdf"
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
	PaymentTotal float64                `json:"payment_total" example:"7500000"`
	Details      []PaymentDetailRequest `json:"details"`
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

// GetAllPayments menangani permintaan untuk mengambil daftar pembayaran & tagihan customer.
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

// GetPayment menangani permintaan untuk mengambil detail pembayaran berdasarkan ID.
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

// CreatePayment menangani pencatatan pembayaran baru dari pelanggan.
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

// UpdatePaymentTotal menangani permintaan pembaruan jumlah total pembayaran.
// UpdatePaymentTotal godoc
// @Summary      Edit pembayaran
// @Description  Memperbarui jumlah total pembayaran (payment_total) beserta alokasi detail order tanpa memperbarui tanggal dan bukti bayar.
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

	if len(req.Details) == 0 {
		return utils.JSONError(c, fiber.StatusBadRequest, "Details alokasi pembayaran tidak boleh kosong")
	}

	userID, err := services.ParseUserID(c.Locals("userID").(string))
	if err != nil {
		return utils.JSONError(c, fiber.StatusUnauthorized, err.Error())
	}

	var orderIDs []string
	for _, detail := range req.Details {
		if detail.OrderID != "" {
			orderIDs = append(orderIDs, detail.OrderID)
		}
	}

	payment, err := services.UpdatePaymentTotal(id, req.PaymentTotal, orderIDs, userID)
	if err != nil {
		if errors.Is(err, services.ErrPaymentInvalidUUID) {
			return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
		}
		if errors.Is(err, services.ErrPaymentNotFound) {
			return utils.JSONError(c, fiber.StatusNotFound, err.Error())
		}
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}

	return utils.JSONSuccess(c, "Pembayaran berhasil diperbarui", payment)
}

// GetCustomerPaymentDetail menangani permintaan mengambil detail lengkap tagihan dan pembayaran satu customer.
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

// GetPaymentHistory menangani permintaan laporan riwayat cicilan transaksi pembayaran.
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

// ExportPaymentHistory mengekspor data riwayat pembayaran ke format Excel atau PDF.
// ExportPaymentHistory godoc
// @Summary      Export Riwayat Pembayaran ke Excel / PDF
// @Description  Mengunduh file Excel atau PDF berisi riwayat alokasi cicilan pembayaran berdasarkan filter pencarian, status pembayaran order terkait, rentang tanggal payments.payment_date, dan parameter format (excel/pdf).
// @Tags         Payments
// @Produce      application/octet-stream
// @Param        search      query  string  false  "Cari nomor PO atau transaksi"
// @Param        status      query  string  false  "Filter status (all, paid, partial)"
// @Param        start_date  query  string  false  "Tanggal awal filter (YYYY-MM-DD)"
// @Param        end_date    query  string  false  "Tanggal akhir filter (YYYY-MM-DD)"
// @Param        format      query  string  false  "Format dokumen (excel/pdf, default: excel)"
// @Success      200  {string}  string  "File Dokumen"
// @Failure      500  {object}  utils.Response
// @Router       /payments/history/export [get]
// @Security     BearerAuth
func ExportPaymentHistory(c *fiber.Ctx) error {
	search := c.Query("search")
	status := c.Query("status", "all")
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")
	format := c.Query("format", "excel")

	history, _, err := services.GetPaymentHistory(search, status, startDate, endDate, 0, 0)
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}

	if format == "pdf" {
		buf, err := generatePaymentHistoryPDF(history, search, status, startDate, endDate)
		if err != nil {
			return utils.JSONError(c, fiber.StatusInternalServerError, "Gagal membuat PDF: "+err.Error())
		}
		c.Set(fiber.HeaderContentType, "application/pdf")
		c.Set(fiber.HeaderContentDisposition, fmt.Sprintf("attachment; filename=\"riwayat-pembayaran-%s.pdf\"", time.Now().Format("2006-01-02-150405")))
		return c.Send(buf.Bytes())
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
	xf.MergeCell(sh, "A2", "B2")
	xf.SetCellValue(sh, "A2", "Waktu Ekspor")
	xf.SetCellStyle(sh, "A2", "B2", st.metaKey)
	xf.SetCellValue(sh, "C2", time.Now().Local().Format("2006-01-02 15:04"))
	xf.SetCellStyle(sh, "C2", "C2", st.metaVal)

	xf.MergeCell(sh, "A3", "B3")
	xf.SetCellValue(sh, "A3", "Filter Pencarian")
	xf.SetCellStyle(sh, "A3", "B3", st.metaKey)
	xf.SetCellValue(sh, "C3", search)
	xf.SetCellStyle(sh, "C3", "C3", st.metaVal)

	xf.MergeCell(sh, "A4", "B4")
	xf.SetCellValue(sh, "A4", "Filter Status")
	xf.SetCellStyle(sh, "A4", "B4", st.metaKey)
	xf.SetCellValue(sh, "C4", status)
	xf.SetCellStyle(sh, "C4", "C4", st.metaVal)

	// Add dropdown data validation in C4
	dv := excelize.NewDataValidation(true)
	dv.Sqref = "C4"
	dv.SetDropList([]string{"all", "paid", "partial"})
	_ = xf.AddDataValidation(sh, dv)

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

	// Enable Excel's built-in AutoFilter
	lastRow := 6 + len(history)
	if lastRow > 6 {
		_ = xf.AutoFilter(sh, fmt.Sprintf("A6:H%d", lastRow), nil)
	}

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

// newPaymentExcelStyles mendefinisikan dan membuat gaya Excel untuk laporan pembayaran.
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
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
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

// setPaymentMetaRow menetapkan nilai dan gaya untuk baris metadata laporan Excel.
func setPaymentMetaRow(f *excelize.File, sh string, row int, label, value string, labelStyle, valStyle int) {
	cell := fmt.Sprintf("A%d", row)
	f.SetCellValue(sh, cell, label)
	f.SetCellStyle(sh, cell, cell, labelStyle)
	v := fmt.Sprintf("B%d", row)
	f.SetCellValue(sh, v, value)
	f.SetCellStyle(sh, v, v, valStyle)
}

// applyPaymentRowStyle menerapkan gaya baris data secara massal ke sel Excel.
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

// generatePaymentHistoryPDF menghasilkan dokumen PDF berisi laporan riwayat pembayaran.
func generatePaymentHistoryPDF(history []services.PaymentHistoryDTO, search, status, startDate, endDate string) (*bytes.Buffer, error) {
	pdf := fpdf.New("L", "mm", "A4", "") // Landscape, A4
	pdf.AddPage()
	pdf.SetMargins(10, 10, 10)

	// Colors
	navyColor := []int{26, 35, 126}      // #1A237E
	darkNavyColor := []int{40, 53, 147}  // #283593
	darkGrey := []int{66, 66, 66}
	textGrey := []int{33, 33, 33}
	borderColor := []int{189, 189, 189}
	zebraColor := []int{232, 234, 246}   // #E8EAF6

	// Title
	pdf.SetFont("Arial", "B", 16)
	pdf.SetTextColor(navyColor[0], navyColor[1], navyColor[2])
	pdf.CellFormat(277, 10, "LAPORAN RIWAYAT PEMBAYARAN", "0", 1, "C", false, 0, "")
	pdf.Ln(4)

	// Metadata
	pdf.SetFont("Arial", "B", 10)
	pdf.SetTextColor(darkGrey[0], darkGrey[1], darkGrey[2])

	// Waktu Ekspor
	pdf.CellFormat(40, 6, "Waktu Ekspor", "0", 0, "L", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	pdf.SetTextColor(textGrey[0], textGrey[1], textGrey[2])
	pdf.CellFormat(100, 6, ": "+time.Now().Local().Format("2006-01-02 15:04"), "0", 1, "L", false, 0, "")

	// Filter Pencarian
	pdf.SetFont("Arial", "B", 10)
	pdf.SetTextColor(darkGrey[0], darkGrey[1], darkGrey[2])
	pdf.CellFormat(40, 6, "Filter Pencarian", "0", 0, "L", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	pdf.SetTextColor(textGrey[0], textGrey[1], textGrey[2])
	if search == "" {
		search = "-"
	}
	pdf.CellFormat(100, 6, ": "+search, "0", 1, "L", false, 0, "")

	// Filter Status
	pdf.SetFont("Arial", "B", 10)
	pdf.SetTextColor(darkGrey[0], darkGrey[1], darkGrey[2])
	pdf.CellFormat(40, 6, "Filter Status", "0", 0, "L", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	pdf.SetTextColor(textGrey[0], textGrey[1], textGrey[2])
	pdf.CellFormat(100, 6, ": "+status, "0", 1, "L", false, 0, "")
	pdf.Ln(6)

	// Table Headers
	colWidths := []float64{10, 45, 40, 60, 35, 37, 30, 20}
	headers := []string{"No", "Nomor Transaksi", "Nomor PO", "Nama Konsumen", "Admin Pembuat", "Tanggal Bayar", "Jumlah Bayar", "Status"}

	pdf.SetFont("Arial", "B", 9)
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFillColor(navyColor[0], navyColor[1], navyColor[2])
	pdf.SetDrawColor(borderColor[0], borderColor[1], borderColor[2])

	for i, h := range headers {
		pdf.CellFormat(colWidths[i], 8, h, "1", 0, "C", true, 0, "")
	}
	pdf.Ln(8)

	// Table Data
	pdf.SetFont("Arial", "", 9)
	pdf.SetTextColor(textGrey[0], textGrey[1], textGrey[2])

	var totalAmt float64
	for i, item := range history {
		fill := false
		if i%2 == 1 {
			pdf.SetFillColor(zebraColor[0], zebraColor[1], zebraColor[2])
			fill = true
		} else {
			pdf.SetFillColor(255, 255, 255)
		}

		pdf.CellFormat(colWidths[0], 7, fmt.Sprintf("%d", i+1), "1", 0, "C", fill, 0, "")
		pdf.CellFormat(colWidths[1], 7, item.TransactionNo, "1", 0, "L", fill, 0, "")
		pdf.CellFormat(colWidths[2], 7, item.PoNo, "1", 0, "L", fill, 0, "")
		pdf.CellFormat(colWidths[3], 7, item.CustomerName, "1", 0, "L", fill, 0, "")
		pdf.CellFormat(colWidths[4], 7, item.AdminName, "1", 0, "L", fill, 0, "")
		pdf.CellFormat(colWidths[5], 7, item.CreatedAt, "1", 0, "C", fill, 0, "")

		formattedAmount := fmt.Sprintf("Rp %.2f", item.TotalAmount)
		pdf.CellFormat(colWidths[6], 7, formattedAmount, "1", 0, "R", fill, 0, "")

		pdf.CellFormat(colWidths[7], 7, item.PaymentStatus, "1", 0, "C", fill, 0, "")
		pdf.Ln(7)

		totalAmt += item.TotalAmount
	}

	// Total Row
	pdf.SetFont("Arial", "B", 9)
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFillColor(darkNavyColor[0], darkNavyColor[1], darkNavyColor[2])

	pdf.CellFormat(227, 8, "TOTAL TERBAYAR", "1", 0, "R", true, 0, "")
	formattedTotal := fmt.Sprintf("Rp %.2f", totalAmt)
	pdf.CellFormat(colWidths[6], 8, formattedTotal, "1", 0, "R", true, 0, "")
	pdf.CellFormat(colWidths[7], 8, "", "1", 1, "C", true, 0, "")

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return &buf, nil
}


