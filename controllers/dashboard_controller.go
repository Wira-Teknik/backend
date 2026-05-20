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
