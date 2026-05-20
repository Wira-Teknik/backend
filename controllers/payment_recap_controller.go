package controllers

import (
	"teknik/services"
	"teknik/utils"

	"github.com/gofiber/fiber/v2"
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
