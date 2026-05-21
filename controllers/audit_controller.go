package controllers

import (
	"teknik/services"
	"teknik/utils"

	"github.com/gofiber/fiber/v2"
)

// ─────────────────────────────────────────────
// Get Audit Logs
// ─────────────────────────────────────────────

// GetAuditLogs godoc
// @Summary      Ambil audit logs
// @Description  Mengambil daftar log audit aktivitas admin. Khusus untuk role owner.
// @Tags         Audits
// @Param        name query string false "Cari nama admin"
// @Produce      json
// @Success      200  {object}  utils.Response{data=[]services.AuditLogDTO}
// @Failure      403  {object}  utils.Response
// @Router       /audit [get]
// @Security     BearerAuth
func GetAuditLogs(c *fiber.Ctx) error {
	// Memastikan user memiliki role owner
	userRole, ok := c.Locals("userRole").(string)
	if !ok || userRole != "owner" {
		return utils.JSONError(c, fiber.StatusForbidden, "Hanya owner yang dapat mengakses audit log")
	}

	name := c.Query("name")
	logs, err := services.GetAuditLogs(name)
	if err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, "Gagal mengambil audit log")
	}

	return utils.JSONSuccess(c, "Audit log berhasil diambil", logs)
}
