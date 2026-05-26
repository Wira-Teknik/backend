package controllers

import (
	"bytes"
	"fmt"
	"teknik/services"
	"teknik/utils"

	"github.com/gofiber/fiber/v2"
	"github.com/xuri/excelize/v2"
)

// ─────────────────────────────────────────────
// Helper: build RecapFilter from query params
// ─────────────────────────────────────────────

func buildRecapFilter(c *fiber.Ctx) services.RecapFilter {
	return services.RecapFilter{
		StartDate: c.Query("start_date"),
		EndDate:   c.Query("end_date"),
		Search:    c.Query("search"),
		Status:    c.Query("status", "all"),
	}
}

// ─────────────────────────────────────────────
// 1. GET /payment-recap
// ─────────────────────────────────────────────

// GetPaymentRecapSummary godoc
// @Summary      Ringkasan Rekapitulasi Pembayaran
// @Description  Menampilkan metrik ringkasan: Total Pendapatan, Total Pesanan, Total Unpaid, Total Paid, dan persentase pelunasan piutang pada periode tertentu.
// @Tags         Payment Recap
// @Produce      json
// @Param        start_date  query  string  false  "Tanggal awal (YYYY-MM-DD)"
// @Param        end_date    query  string  false  "Tanggal akhir (YYYY-MM-DD)"
// @Param        search      query  string  false  "Cari berdasarkan nama konsumen"
// @Success      200  {object}  utils.Response{data=services.RecapSummaryDTO}
// @Failure      500  {object}  utils.Response
// @Router       /payment-recap [get]
// @Security     BearerAuth
func GetPaymentRecapSummary(c *fiber.Ctx) error {
	f := buildRecapFilter(c)
	data, err := services.GetPaymentRecapSummary(f)
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}
	return utils.JSONSuccess(c, "Ringkasan rekapitulasi berhasil diambil", data)
}

// ─────────────────────────────────────────────
// 2. GET /payment-recap/detail-pendapatan
// ─────────────────────────────────────────────

// GetDetailPendapatan godoc
// @Summary      Detail Total Pendapatan
// @Description  Menampilkan daftar invoice berdasarkan periode dan filter status pembayaran (all, paid, unpaid).
// @Tags         Payment Recap
// @Produce      json
// @Param        start_date  query  string  false  "Tanggal awal (YYYY-MM-DD)"
// @Param        end_date    query  string  false  "Tanggal akhir (YYYY-MM-DD)"
// @Param        search      query  string  false  "Cari berdasarkan nama konsumen"
// @Param        status      query  string  false  "Filter status: all, paid, unpaid"
// @Success      200  {object}  utils.Response{data=services.DetailPendapatanDTO}
// @Failure      500  {object}  utils.Response
// @Router       /payment-recap/detail-pendapatan [get]
// @Security     BearerAuth
func GetDetailPendapatan(c *fiber.Ctx) error {
	f := buildRecapFilter(c)
	data, err := services.GetDetailPendapatan(f)
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}
	return utils.JSONSuccess(c, "Detail pendapatan berhasil diambil", data)
}

// ─────────────────────────────────────────────
// 3. GET /payment-recap/detail-pesanan
// ─────────────────────────────────────────────

// GetDetailPesanan godoc
// @Summary      Detail Total Pesanan
// @Description  Menampilkan daftar pesanan berdasarkan periode dan filter status (all, pending, partial, shipped, completed).
// @Tags         Payment Recap
// @Produce      json
// @Param        start_date  query  string  false  "Tanggal awal (YYYY-MM-DD)"
// @Param        end_date    query  string  false  "Tanggal akhir (YYYY-MM-DD)"
// @Param        search      query  string  false  "Cari berdasarkan nama konsumen atau PO"
// @Param        status      query  string  false  "Filter status: all, pending, partial, shipped, completed"
// @Success      200  {object}  utils.Response{data=services.DetailPesananDTO}
// @Failure      500  {object}  utils.Response
// @Router       /payment-recap/detail-pesanan [get]
// @Security     BearerAuth
func GetDetailPesanan(c *fiber.Ctx) error {
	f := buildRecapFilter(c)
	data, err := services.GetDetailPesanan(f)
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}
	return utils.JSONSuccess(c, "Detail pesanan berhasil diambil", data)
}

// ─────────────────────────────────────────────
// 4. GET /payment-recap/detail-unpaid
// ─────────────────────────────────────────────

// GetDetailUnpaid godoc
// @Summary      Detail Total Unpaid
// @Description  Menampilkan daftar invoice yang belum/sebagian lunas pada periode tertentu.
// @Tags         Payment Recap
// @Produce      json
// @Param        start_date  query  string  false  "Tanggal awal (YYYY-MM-DD)"
// @Param        end_date    query  string  false  "Tanggal akhir (YYYY-MM-DD)"
// @Param        search      query  string  false  "Cari berdasarkan nama konsumen atau PO"
// @Success      200  {object}  utils.Response{data=services.DetailUnpaidDTO}
// @Failure      500  {object}  utils.Response
// @Router       /payment-recap/detail-unpaid [get]
// @Security     BearerAuth
func GetDetailUnpaid(c *fiber.Ctx) error {
	f := buildRecapFilter(c)
	data, err := services.GetDetailUnpaid(f)
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}
	return utils.JSONSuccess(c, "Detail unpaid berhasil diambil", data)
}

// ─────────────────────────────────────────────
// 5. GET /payment-recap/detail-paid
// ─────────────────────────────────────────────

// GetDetailPaid godoc
// @Summary      Detail Total Paid
// @Description  Menampilkan daftar invoice yang sudah lunas pada periode tertentu.
// @Tags         Payment Recap
// @Produce      json
// @Param        start_date  query  string  false  "Tanggal awal (YYYY-MM-DD)"
// @Param        end_date    query  string  false  "Tanggal akhir (YYYY-MM-DD)"
// @Param        search      query  string  false  "Cari berdasarkan nama konsumen atau PO"
// @Success      200  {object}  utils.Response{data=services.DetailPaidDTO}
// @Failure      500  {object}  utils.Response
// @Router       /payment-recap/detail-paid [get]
// @Security     BearerAuth
func GetDetailPaid(c *fiber.Ctx) error {
	f := buildRecapFilter(c)
	data, err := services.GetDetailPaid(f)
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}
	return utils.JSONSuccess(c, "Detail paid berhasil diambil", data)
}

// ─────────────────────────────────────────────
// SHARED EXCEL HELPER
// ─────────────────────────────────────────────

// excelStyles menyimpan ID style excelize yang dipakai bersama.
type excelStyles struct {
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

func newExcelStyles(f *excelize.File) (excelStyles, error) {
	var s excelStyles
	var err error

	// Border thin
	border := []excelize.Border{
		{Type: "left", Color: "BDBDBD", Style: 1},
		{Type: "right", Color: "BDBDBD", Style: 1},
		{Type: "top", Color: "BDBDBD", Style: 1},
		{Type: "bottom", Color: "BDBDBD", Style: 1},
	}

	// ── Title ──
	s.titleStyle, err = f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 16, Color: "1A237E", Family: "Calibri"},
		Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "center"},
	})
	if err != nil {
		return s, err
	}

	// ── Meta label (kiri) ──
	s.metaKey, err = f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 10, Color: "424242", Family: "Calibri"},
		Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "center"},
	})
	if err != nil {
		return s, err
	}

	// ── Meta value (kanan) ──
	s.metaVal, err = f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Size: 10, Color: "212121", Family: "Calibri"},
		Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "center"},
	})
	if err != nil {
		return s, err
	}

	// ── Header tabel ──
	s.headerStyle, err = f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 10, Color: "FFFFFF", Family: "Calibri"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"1A237E"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
		Border:    border,
	})
	if err != nil {
		return s, err
	}

	// ── Data row normal ──
	s.dataStyle, err = f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Size: 10, Color: "212121", Family: "Calibri"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"FFFFFF"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "center"},
		Border:    border,
	})
	if err != nil {
		return s, err
	}

	// ── Data row alternating ──
	s.dataAlt, err = f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Size: 10, Color: "212121", Family: "Calibri"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"E8EAF6"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "center"},
		Border:    border,
	})
	if err != nil {
		return s, err
	}

	// ── Currency normal ──
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

	// ── Currency alternating ──
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

	// ── Summary label ──
	s.sumLabel, err = f.NewStyle(&excelize.Style{
		Font:   &excelize.Font{Bold: true, Size: 10, Color: "FFFFFF", Family: "Calibri"},
		Fill:   excelize.Fill{Type: "pattern", Color: []string{"283593"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "right", Vertical: "center"},
		Border: border,
	})
	if err != nil {
		return s, err
	}

	// ── Summary value ──
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

func strPtr(s string) *string { return &s }

// setMetaRow menulis baris meta label + value pada kolom A dan B.
func setMetaRow(f *excelize.File, sh string, row int, label, value string, labelStyle, valStyle int) {
	cell := fmt.Sprintf("A%d", row)
	f.SetCellValue(sh, cell, label)
	f.SetCellStyle(sh, cell, cell, labelStyle)
	v := fmt.Sprintf("B%d", row)
	f.SetCellValue(sh, v, value)
	f.SetCellStyle(sh, v, v, valStyle)
}

// applyRowStyle menerapkan style pada semua sel data dalam satu baris tabel.
func applyRowStyle(f *excelize.File, sh string, row, numCols int, textSt, numSt int, numCol int) {
	for col := 1; col <= numCols; col++ {
		cellRef, _ := excelize.CoordinatesToCellName(col, row)
		if col == numCol {
			f.SetCellStyle(sh, cellRef, cellRef, numSt)
		} else {
			f.SetCellStyle(sh, cellRef, cellRef, textSt)
		}
	}
}

// ─────────────────────────────────────────────
// EXPORT TO EXCEL HANDLERS
// ─────────────────────────────────────────────

// ExportDetailPendapatan godoc
// @Summary      Export Detail Total Pendapatan ke Excel
// @Description  Mengunduh file Excel berisi detail total pendapatan berdasarkan periode dan filter status pembayaran.
// @Tags         Payment Recap
// @Produce      application/vnd.openxmlformats-officedocument.spreadsheetml.sheet
// @Param        start_date  query  string  false  "Tanggal awal (YYYY-MM-DD)"
// @Param        end_date    query  string  false  "Tanggal akhir (YYYY-MM-DD)"
// @Param        search      query  string  false  "Cari berdasarkan nama konsumen"
// @Param        status      query  string  false  "Filter status: all, paid, unpaid"
// @Success      200  {string}  string  "File Excel"
// @Failure      500  {object}  utils.Response
// @Router       /payment-recap/detail-pendapatan/export [get]
// @Security     BearerAuth
func ExportDetailPendapatan(c *fiber.Ctx) error {
	f := buildRecapFilter(c)
	data, err := services.GetDetailPendapatan(f)
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}

	xf := excelize.NewFile()
	defer xf.Close()
	sh := "Detail Pendapatan"
	xf.SetSheetName("Sheet1", sh)

	st, err := newExcelStyles(xf)
	if err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}

	// ── Title ──
	xf.MergeCell(sh, "A1", "G1")
	xf.SetCellValue(sh, "A1", "LAPORAN DETAIL PENDAPATAN")
	xf.SetCellStyle(sh, "A1", "G1", st.titleStyle)
	xf.SetRowHeight(sh, 1, 28)

	// ── Meta summary ──
	setMetaRow(xf, sh, 2, "Periode Waktu", fmt.Sprintf("%s s/d %s", f.StartDate, f.EndDate), st.metaKey, st.metaVal)
	setMetaRow(xf, sh, 3, "Total Pendapatan", fmt.Sprintf("Rp %.2f", data.TotalPendapatan), st.metaKey, st.metaVal)
	setMetaRow(xf, sh, 4, "Total Pesanan", fmt.Sprintf("%d Orders", data.TotalPesananCount), st.metaKey, st.metaVal)

	// ── Header row ──
	headers := []string{"No", "Nomor Transaksi", "Nomor PO", "Nama Konsumen", "Tanggal", "Jumlah Total", "Status Pembayaran"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 6)
		xf.SetCellValue(sh, cell, h)
		xf.SetCellStyle(sh, cell, cell, st.headerStyle)
	}
	xf.SetRowHeight(sh, 6, 22)

	// ── Data rows ──
	var totalAmt float64
	for i, item := range data.Items {
		row := 7 + i
		textSt, numSt := st.dataStyle, st.currency
		if i%2 == 1 {
			textSt, numSt = st.dataAlt, st.currencyAlt
		}
		xf.SetCellInt(sh, fmt.Sprintf("A%d", row), int64(i+1))
		xf.SetCellValue(sh, fmt.Sprintf("B%d", row), item.TransactionNo)
		xf.SetCellValue(sh, fmt.Sprintf("C%d", row), item.PoNo)
		xf.SetCellValue(sh, fmt.Sprintf("D%d", row), item.CustomerName)
		xf.SetCellValue(sh, fmt.Sprintf("E%d", row), item.Date)
		xf.SetCellFloat(sh, fmt.Sprintf("F%d", row), item.TotalAmount, 2, 64)
		xf.SetCellValue(sh, fmt.Sprintf("G%d", row), item.PaymentStatus)
		applyRowStyle(xf, sh, row, 7, textSt, numSt, 6)
		xf.SetRowHeight(sh, row, 18)
		totalAmt += item.TotalAmount
	}

	// ── Summary/total row ──
	sumRow := 7 + len(data.Items)
	xf.MergeCell(sh, fmt.Sprintf("A%d", sumRow), fmt.Sprintf("E%d", sumRow))
	xf.SetCellValue(sh, fmt.Sprintf("A%d", sumRow), "TOTAL")
	xf.SetCellStyle(sh, fmt.Sprintf("A%d", sumRow), fmt.Sprintf("E%d", sumRow), st.sumLabel)
	xf.SetCellFloat(sh, fmt.Sprintf("F%d", sumRow), totalAmt, 2, 64)
	xf.SetCellStyle(sh, fmt.Sprintf("F%d", sumRow), fmt.Sprintf("F%d", sumRow), st.sumValue)
	xf.SetCellStyle(sh, fmt.Sprintf("G%d", sumRow), fmt.Sprintf("G%d", sumRow), st.sumLabel)
	xf.SetRowHeight(sh, sumRow, 20)

	// ── Column widths ──
	xf.SetColWidth(sh, "A", "A", 5)
	xf.SetColWidth(sh, "B", "B", 18)
	xf.SetColWidth(sh, "C", "C", 16)
	xf.SetColWidth(sh, "D", "D", 30)
	xf.SetColWidth(sh, "E", "E", 14)
	xf.SetColWidth(sh, "F", "F", 20)
	xf.SetColWidth(sh, "G", "G", 18)

	buf := new(bytes.Buffer)
	if err := xf.Write(buf); err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}

	c.Set(fiber.HeaderContentType, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Set(fiber.HeaderContentDisposition, fmt.Sprintf("attachment; filename=\"detail-pendapatan-%s-%s.xlsx\"", f.StartDate, f.EndDate))
	return c.Send(buf.Bytes())
}

// ExportDetailPesanan godoc
// @Summary      Export Detail Total Pesanan ke Excel
// @Description  Mengunduh file Excel berisi detail total pesanan berdasarkan periode dan filter status.
// @Tags         Payment Recap
// @Produce      application/vnd.openxmlformats-officedocument.spreadsheetml.sheet
// @Param        start_date  query  string  false  "Tanggal awal (YYYY-MM-DD)"
// @Param        end_date    query  string  false  "Tanggal akhir (YYYY-MM-DD)"
// @Param        search      query  string  false  "Cari berdasarkan nama konsumen atau PO"
// @Param        status      query  string  false  "Filter status: all, pending, partial, shipped, completed"
// @Success      200  {string}  string  "File Excel"
// @Failure      500  {object}  utils.Response
// @Router       /payment-recap/detail-pesanan/export [get]
// @Security     BearerAuth
func ExportDetailPesanan(c *fiber.Ctx) error {
	f := buildRecapFilter(c)
	data, err := services.GetDetailPesanan(f)
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}

	xf := excelize.NewFile()
	defer xf.Close()
	sh := "Detail Pesanan"
	xf.SetSheetName("Sheet1", sh)

	st, err := newExcelStyles(xf)
	if err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}

	xf.MergeCell(sh, "A1", "G1")
	xf.SetCellValue(sh, "A1", "LAPORAN DETAIL TOTAL PESANAN")
	xf.SetCellStyle(sh, "A1", "G1", st.titleStyle)
	xf.SetRowHeight(sh, 1, 28)

	setMetaRow(xf, sh, 2, "Periode Waktu", fmt.Sprintf("%s s/d %s", f.StartDate, f.EndDate), st.metaKey, st.metaVal)
	setMetaRow(xf, sh, 3, "Total Nominal Pesanan", fmt.Sprintf("Rp %.2f", data.TotalPesananAmount), st.metaKey, st.metaVal)
	setMetaRow(xf, sh, 4, "Total Pesanan", fmt.Sprintf("%d Orders", data.TotalPesananCount), st.metaKey, st.metaVal)

	headers := []string{"No", "Nomor Transaksi", "Nomor PO", "Nama Konsumen", "Tanggal", "Jumlah Total", "Status Pesanan"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 6)
		xf.SetCellValue(sh, cell, h)
		xf.SetCellStyle(sh, cell, cell, st.headerStyle)
	}
	xf.SetRowHeight(sh, 6, 22)

	var totalAmt float64
	for i, item := range data.Items {
		row := 7 + i
		textSt, numSt := st.dataStyle, st.currency
		if i%2 == 1 {
			textSt, numSt = st.dataAlt, st.currencyAlt
		}
		xf.SetCellInt(sh, fmt.Sprintf("A%d", row), int64(i+1))
		xf.SetCellValue(sh, fmt.Sprintf("B%d", row), item.TransactionNo)
		xf.SetCellValue(sh, fmt.Sprintf("C%d", row), item.PoNo)
		xf.SetCellValue(sh, fmt.Sprintf("D%d", row), item.CustomerName)
		xf.SetCellValue(sh, fmt.Sprintf("E%d", row), item.Date)
		xf.SetCellFloat(sh, fmt.Sprintf("F%d", row), item.TotalAmount, 2, 64)
		xf.SetCellValue(sh, fmt.Sprintf("G%d", row), item.OrderStatus)
		applyRowStyle(xf, sh, row, 7, textSt, numSt, 6)
		xf.SetRowHeight(sh, row, 18)
		totalAmt += item.TotalAmount
	}

	sumRow := 7 + len(data.Items)
	xf.MergeCell(sh, fmt.Sprintf("A%d", sumRow), fmt.Sprintf("E%d", sumRow))
	xf.SetCellValue(sh, fmt.Sprintf("A%d", sumRow), "TOTAL")
	xf.SetCellStyle(sh, fmt.Sprintf("A%d", sumRow), fmt.Sprintf("E%d", sumRow), st.sumLabel)
	xf.SetCellFloat(sh, fmt.Sprintf("F%d", sumRow), totalAmt, 2, 64)
	xf.SetCellStyle(sh, fmt.Sprintf("F%d", sumRow), fmt.Sprintf("F%d", sumRow), st.sumValue)
	xf.SetCellStyle(sh, fmt.Sprintf("G%d", sumRow), fmt.Sprintf("G%d", sumRow), st.sumLabel)
	xf.SetRowHeight(sh, sumRow, 20)

	xf.SetColWidth(sh, "A", "A", 5)
	xf.SetColWidth(sh, "B", "B", 18)
	xf.SetColWidth(sh, "C", "C", 16)
	xf.SetColWidth(sh, "D", "D", 30)
	xf.SetColWidth(sh, "E", "E", 14)
	xf.SetColWidth(sh, "F", "F", 20)
	xf.SetColWidth(sh, "G", "G", 18)

	buf := new(bytes.Buffer)
	if err := xf.Write(buf); err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}

	c.Set(fiber.HeaderContentType, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Set(fiber.HeaderContentDisposition, fmt.Sprintf("attachment; filename=\"detail-pesanan-%s-%s.xlsx\"", f.StartDate, f.EndDate))
	return c.Send(buf.Bytes())
}

// ExportDetailUnpaid godoc
// @Summary      Export Detail Total Unpaid ke Excel
// @Description  Mengunduh file Excel berisi detail total unpaid berdasarkan periode.
// @Tags         Payment Recap
// @Produce      application/vnd.openxmlformats-officedocument.spreadsheetml.sheet
// @Param        start_date  query  string  false  "Tanggal awal (YYYY-MM-DD)"
// @Param        end_date    query  string  false  "Tanggal akhir (YYYY-MM-DD)"
// @Param        search      query  string  false  "Cari berdasarkan nama konsumen atau PO"
// @Success      200  {string}  string  "File Excel"
// @Failure      500  {object}  utils.Response
// @Router       /payment-recap/detail-unpaid/export [get]
// @Security     BearerAuth
func ExportDetailUnpaid(c *fiber.Ctx) error {
	f := buildRecapFilter(c)
	data, err := services.GetDetailUnpaid(f)
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}

	xf := excelize.NewFile()
	defer xf.Close()
	sh := "Detail Unpaid"
	xf.SetSheetName("Sheet1", sh)

	st, err := newExcelStyles(xf)
	if err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}

	xf.MergeCell(sh, "A1", "G1")
	xf.SetCellValue(sh, "A1", "LAPORAN DETAIL TOTAL UNPAID")
	xf.SetCellStyle(sh, "A1", "G1", st.titleStyle)
	xf.SetRowHeight(sh, 1, 28)

	setMetaRow(xf, sh, 2, "Periode Waktu", fmt.Sprintf("%s s/d %s", f.StartDate, f.EndDate), st.metaKey, st.metaVal)
	setMetaRow(xf, sh, 3, "Total Unpaid", fmt.Sprintf("Rp %.2f", data.TotalUnpaid), st.metaKey, st.metaVal)
	setMetaRow(xf, sh, 4, "Jumlah Invoice Unpaid/Partial", fmt.Sprintf("%d Invoices", data.TotalCount), st.metaKey, st.metaVal)
	setMetaRow(xf, sh, 5, "Total Pesanan Periode", fmt.Sprintf("%d Orders", data.TotalPesananCount), st.metaKey, st.metaVal)

	headers := []string{"No", "Nomor Transaksi", "Nomor PO", "Nama Konsumen", "Tanggal", "Sisa Tagihan", "Status Pembayaran"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 7)
		xf.SetCellValue(sh, cell, h)
		xf.SetCellStyle(sh, cell, cell, st.headerStyle)
	}
	xf.SetRowHeight(sh, 7, 22)

	var totalAmt float64
	for i, item := range data.Items {
		row := 8 + i
		textSt, numSt := st.dataStyle, st.currency
		if i%2 == 1 {
			textSt, numSt = st.dataAlt, st.currencyAlt
		}
		xf.SetCellInt(sh, fmt.Sprintf("A%d", row), int64(i+1))
		xf.SetCellValue(sh, fmt.Sprintf("B%d", row), item.TransactionNo)
		xf.SetCellValue(sh, fmt.Sprintf("C%d", row), item.PoNo)
		xf.SetCellValue(sh, fmt.Sprintf("D%d", row), item.CustomerName)
		xf.SetCellValue(sh, fmt.Sprintf("E%d", row), item.Date)
		xf.SetCellFloat(sh, fmt.Sprintf("F%d", row), item.TotalAmount, 2, 64)
		xf.SetCellValue(sh, fmt.Sprintf("G%d", row), item.PaymentStatus)
		applyRowStyle(xf, sh, row, 7, textSt, numSt, 6)
		xf.SetRowHeight(sh, row, 18)
		totalAmt += item.TotalAmount
	}

	sumRow := 8 + len(data.Items)
	xf.MergeCell(sh, fmt.Sprintf("A%d", sumRow), fmt.Sprintf("E%d", sumRow))
	xf.SetCellValue(sh, fmt.Sprintf("A%d", sumRow), "TOTAL SISA TAGIHAN")
	xf.SetCellStyle(sh, fmt.Sprintf("A%d", sumRow), fmt.Sprintf("E%d", sumRow), st.sumLabel)
	xf.SetCellFloat(sh, fmt.Sprintf("F%d", sumRow), totalAmt, 2, 64)
	xf.SetCellStyle(sh, fmt.Sprintf("F%d", sumRow), fmt.Sprintf("F%d", sumRow), st.sumValue)
	xf.SetCellStyle(sh, fmt.Sprintf("G%d", sumRow), fmt.Sprintf("G%d", sumRow), st.sumLabel)
	xf.SetRowHeight(sh, sumRow, 20)

	xf.SetColWidth(sh, "A", "A", 5)
	xf.SetColWidth(sh, "B", "B", 18)
	xf.SetColWidth(sh, "C", "C", 16)
	xf.SetColWidth(sh, "D", "D", 30)
	xf.SetColWidth(sh, "E", "E", 14)
	xf.SetColWidth(sh, "F", "F", 20)
	xf.SetColWidth(sh, "G", "G", 18)

	buf := new(bytes.Buffer)
	if err := xf.Write(buf); err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}

	c.Set(fiber.HeaderContentType, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Set(fiber.HeaderContentDisposition, fmt.Sprintf("attachment; filename=\"detail-unpaid-%s-%s.xlsx\"", f.StartDate, f.EndDate))
	return c.Send(buf.Bytes())
}

// ExportDetailPaid godoc
// @Summary      Export Detail Total Paid ke Excel
// @Description  Mengunduh file Excel berisi detail total paid berdasarkan periode.
// @Tags         Payment Recap
// @Produce      application/vnd.openxmlformats-officedocument.spreadsheetml.sheet
// @Param        start_date  query  string  false  "Tanggal awal (YYYY-MM-DD)"
// @Param        end_date    query  string  false  "Tanggal akhir (YYYY-MM-DD)"
// @Param        search      query  string  false  "Cari berdasarkan nama konsumen atau PO"
// @Success      200  {string}  string  "File Excel"
// @Failure      500  {object}  utils.Response
// @Router       /payment-recap/detail-paid/export [get]
// @Security     BearerAuth
func ExportDetailPaid(c *fiber.Ctx) error {
	f := buildRecapFilter(c)
	data, err := services.GetDetailPaid(f)
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}

	xf := excelize.NewFile()
	defer xf.Close()
	sh := "Detail Paid"
	xf.SetSheetName("Sheet1", sh)

	st, err := newExcelStyles(xf)
	if err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}

	xf.MergeCell(sh, "A1", "G1")
	xf.SetCellValue(sh, "A1", "LAPORAN DETAIL TOTAL PAID")
	xf.SetCellStyle(sh, "A1", "G1", st.titleStyle)
	xf.SetRowHeight(sh, 1, 28)

	setMetaRow(xf, sh, 2, "Periode Waktu", fmt.Sprintf("%s s/d %s", f.StartDate, f.EndDate), st.metaKey, st.metaVal)
	setMetaRow(xf, sh, 3, "Total Paid", fmt.Sprintf("Rp %.2f", data.TotalPaid), st.metaKey, st.metaVal)
	setMetaRow(xf, sh, 4, "Jumlah Invoice Paid", fmt.Sprintf("%d Invoices", data.TotalCount), st.metaKey, st.metaVal)
	setMetaRow(xf, sh, 5, "Total Pesanan Periode", fmt.Sprintf("%d Orders", data.TotalPesananCount), st.metaKey, st.metaVal)

	headers := []string{"No", "Nomor Transaksi", "Nomor PO", "Nama Konsumen", "Tanggal", "Jumlah Lunas", "Status Pembayaran"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 7)
		xf.SetCellValue(sh, cell, h)
		xf.SetCellStyle(sh, cell, cell, st.headerStyle)
	}
	xf.SetRowHeight(sh, 7, 22)

	var totalAmt float64
	for i, item := range data.Items {
		row := 8 + i
		textSt, numSt := st.dataStyle, st.currency
		if i%2 == 1 {
			textSt, numSt = st.dataAlt, st.currencyAlt
		}
		xf.SetCellInt(sh, fmt.Sprintf("A%d", row), int64(i+1))
		xf.SetCellValue(sh, fmt.Sprintf("B%d", row), item.TransactionNo)
		xf.SetCellValue(sh, fmt.Sprintf("C%d", row), item.PoNo)
		xf.SetCellValue(sh, fmt.Sprintf("D%d", row), item.CustomerName)
		xf.SetCellValue(sh, fmt.Sprintf("E%d", row), item.Date)
		xf.SetCellFloat(sh, fmt.Sprintf("F%d", row), item.TotalAmount, 2, 64)
		xf.SetCellValue(sh, fmt.Sprintf("G%d", row), item.PaymentStatus)
		applyRowStyle(xf, sh, row, 7, textSt, numSt, 6)
		xf.SetRowHeight(sh, row, 18)
		totalAmt += item.TotalAmount
	}

	sumRow := 8 + len(data.Items)
	xf.MergeCell(sh, fmt.Sprintf("A%d", sumRow), fmt.Sprintf("E%d", sumRow))
	xf.SetCellValue(sh, fmt.Sprintf("A%d", sumRow), "TOTAL LUNAS")
	xf.SetCellStyle(sh, fmt.Sprintf("A%d", sumRow), fmt.Sprintf("E%d", sumRow), st.sumLabel)
	xf.SetCellFloat(sh, fmt.Sprintf("F%d", sumRow), totalAmt, 2, 64)
	xf.SetCellStyle(sh, fmt.Sprintf("F%d", sumRow), fmt.Sprintf("F%d", sumRow), st.sumValue)
	xf.SetCellStyle(sh, fmt.Sprintf("G%d", sumRow), fmt.Sprintf("G%d", sumRow), st.sumLabel)
	xf.SetRowHeight(sh, sumRow, 20)

	xf.SetColWidth(sh, "A", "A", 5)
	xf.SetColWidth(sh, "B", "B", 18)
	xf.SetColWidth(sh, "C", "C", 16)
	xf.SetColWidth(sh, "D", "D", 30)
	xf.SetColWidth(sh, "E", "E", 14)
	xf.SetColWidth(sh, "F", "F", 20)
	xf.SetColWidth(sh, "G", "G", 18)

	buf := new(bytes.Buffer)
	if err := xf.Write(buf); err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, err.Error())
	}

	c.Set(fiber.HeaderContentType, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Set(fiber.HeaderContentDisposition, fmt.Sprintf("attachment; filename=\"detail-paid-%s-%s.xlsx\"", f.StartDate, f.EndDate))
	return c.Send(buf.Bytes())
}
