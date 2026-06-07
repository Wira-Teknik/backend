package controllers

import (
	"errors"

	"teknik/services"
	"teknik/utils"

	"github.com/gofiber/fiber/v2"
)

// ─────────────────────────────────────────────
// Get All Invoices
// ─────────────────────────────────────────────

// GetAllInvoices godoc
// @Summary      Ambil semua invoice
// @Description  Mengambil daftar semua invoice. Invoice otomatis dibuat saat pengiriman dibuat.
// @Tags         Invoices
// @Produce      json
// @Success      200  {object}  utils.Response{data=[]models.Invoice}
// @Router       /invoices [get]
// @Security     BearerAuth
func GetAllInvoices(c *fiber.Ctx) error {
	invoices, err := services.GetAllInvoices()
	if err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, "Gagal mengambil data invoice")
	}
	return utils.JSONSuccess(c, "Data invoice berhasil diambil", invoices)
}

// ─────────────────────────────────────────────
// Get Invoice by ID
// ─────────────────────────────────────────────

// GetInvoice godoc
// @Summary      Ambil detail invoice
// @Description  Mengambil detail invoice berdasarkan ID
// @Tags         Invoices
// @Param        id   path      string  true  "Invoice ID"
// @Produce      json
// @Success      200  {object}  utils.Response{data=models.Invoice}
// @Failure      404  {object}  utils.Response
// @Router       /invoices/{id} [get]
// @Security     BearerAuth
func GetInvoice(c *fiber.Ctx) error {
	id := c.Params("id")
	invoice, err := services.GetInvoiceByID(id)
	if err != nil {
		if errors.Is(err, services.ErrInvoiceInvalidUUID) {
			return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
		}
		if errors.Is(err, services.ErrInvoiceNotFound) {
			return utils.JSONError(c, fiber.StatusNotFound, err.Error())
		}
		return utils.JSONError(c, fiber.StatusInternalServerError, "Gagal mengambil detail invoice")
	}
	return utils.JSONSuccess(c, "Detail invoice berhasil diambil", invoice)
}

// ─────────────────────────────────────────────
// Get Invoice by Shipment ID
// ─────────────────────────────────────────────

// GetInvoiceByShipment godoc
// @Summary      Ambil invoice berdasarkan Shipment
// @Description  Mengambil invoice yang terkait dengan pengiriman tertentu
// @Tags         Shipments
// @Param        shipmentId  path      string  true  "Shipment ID"
// @Produce      json
// @Success      200  {object}  utils.Response{data=models.Invoice}
// @Failure      404  {object}  utils.Response
// @Router       /shipments/{shipmentId}/invoice [get]
// @Security     BearerAuth
func GetInvoiceByShipment(c *fiber.Ctx) error {
	shipmentID := c.Params("shipmentId")
	invoice, err := services.GetInvoiceByShipmentID(shipmentID)
	if err != nil {
		if errors.Is(err, services.ErrInvoiceInvalidShipmentID) {
			return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
		}
		if errors.Is(err, services.ErrInvoiceNotFound) {
			return utils.JSONError(c, fiber.StatusNotFound, err.Error())
		}
		return utils.JSONError(c, fiber.StatusInternalServerError, "Gagal mengambil invoice")
	}
	return utils.JSONSuccess(c, "Invoice berhasil diambil", invoice)
}
