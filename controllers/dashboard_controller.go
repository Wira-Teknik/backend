package controllers

import (
	"teknik/services"
	"teknik/utils"

	"github.com/gofiber/fiber/v2"
)

// ─────────────────────────────────────────────
// Dashboard
// ─────────────────────────────────────────────

// GetDashboard godoc
// @Summary      Ambil Metrik Dashboard
// @Description  Mengambil agregasi metrik untuk keperluan halaman Dashboard (mendukung format untuk Admin maupun Owner).
// @Tags         Dashboard
// @Produce      json
// @Success      200  {object}  utils.Response{data=services.DashboardResponseDTO}
// @Failure      500  {object}  utils.Response
// @Router       /dashboard [get]
// @Security     BearerAuth
func GetDashboard(c *fiber.Ctx) error {
	metrics, err := services.GetDashboardMetrics()
	if err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, "Gagal mengambil metrik dashboard: "+err.Error())
	}
	return utils.JSONSuccess(c, "Metrik dashboard berhasil diambil", metrics)
}

// PaginatedActivitiesResponse adalah wrapper response paginasi untuk daftar aktivitas dashboard.
type PaginatedActivitiesResponse struct {
	Success    bool                           `json:"success"    example:"true"`
	Message    string                         `json:"message"    example:"Semua aktivitas dashboard berhasil diambil"`
	Data       []services.DashboardActivityDTO `json:"data"`
	Pagination PaginationMeta                 `json:"pagination"`
}

// GetAllDashboardActivities godoc
// @Summary      Ambil Semua Aktivitas Terakhir Dashboard
// @Description  Mengambil daftar lengkap seluruh aktivitas terbaru (pengiriman, pemesanan, dan pembayaran) untuk disajikan pada layar "Lihat Semua" Aktivitas Terakhir di Dashboard dengan dukungan pagination dan filter rentang tanggal.
// @Tags         Dashboard
// @Produce      json
// @Param        start_date      query  string  false  "Tanggal awal filter (YYYY-MM-DD)"
// @Param        end_date        query  string  false  "Tanggal akhir filter (YYYY-MM-DD)"
// @Param        page            query  int     false  "Halaman aktif (default: 1)"
// @Param        limit           query  int     false  "Jumlah baris per halaman (default: 20)"
// @Success      200  {object}  controllers.PaginatedActivitiesResponse
// @Failure      400  {object}  utils.Response
// @Failure      500  {object}  utils.Response
// @Router       /dashboard/activities [get]
// @Security     BearerAuth
func GetAllDashboardActivities(c *fiber.Ctx) error {
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")
	page, limit := parsePaginationParams(c)
	activities, totalRows, err := services.GetAllDashboardActivities(startDate, endDate, page, limit)
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}

	totalPages := 0
	if totalRows > 0 && limit > 0 {
		totalPages = int((totalRows + int64(limit) - 1) / int64(limit))
	}

	return c.Status(fiber.StatusOK).JSON(PaginatedActivitiesResponse{
		Success: true,
		Message: "Semua aktivitas dashboard berhasil diambil",
		Data:    activities,
		Pagination: PaginationMeta{
			Page:       page,
			Limit:      limit,
			TotalRows:  totalRows,
			TotalPages: totalPages,
		},
	})
}

