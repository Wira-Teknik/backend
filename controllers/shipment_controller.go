package controllers

import (
	"encoding/json"
	"teknik/services"
	"teknik/utils"

	"github.com/gofiber/fiber/v2"
)

// ─────────────────────────────────────────────
// Request DTOs (exported for Swagger)
// ─────────────────────────────────────────────

// CreateShipmentRequest adalah body untuk membuat pengiriman.
type CreateShipmentRequest struct {
	OrderID      string                       `json:"order_id"      example:"550e8400-e29b-41d4-a716-446655440000"`
	ShippingDate string                       `json:"shipping_date" example:"2026-05-10"`
	Items        []ShipmentItemRequestPayload `json:"items"`
}

// ShipmentItemRequestPayload adalah detail item dalam pengiriman.
type ShipmentItemRequestPayload struct {
	OrderItemID string `json:"order_item_id" example:"550e8400-e29b-41d4-a716-446655440001"`
	ShippingQty int    `json:"shipping_qty"  example:"50"`
}

// ─────────────────────────────────────────────
// Get Shipments by Order
// ─────────────────────────────────────────────

// GetShipmentsByOrder godoc
// @Summary      Ambil semua pengiriman berdasarkan Order
// @Description  Mengambil daftar pengiriman beserta item-nya untuk order tertentu
// @Tags         Shipments
// @Param        orderId  path      string  true  "Order ID"
// @Produce      json
// @Success      200  {object}  utils.Response{data=[]models.Shipment}
// @Router       /orders/{orderId}/shipments [get]
// @Security     BearerAuth
func GetShipmentsByOrder(c *fiber.Ctx) error {
	orderID := c.Params("orderId")
	shipments, err := services.GetShipmentsByOrderID(orderID)
	if err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, "Gagal mengambil data pengiriman")
	}
	return utils.JSONSuccess(c, "Data pengiriman berhasil diambil", shipments)
}

// ─────────────────────────────────────────────
// Get Shipment Detail
// ─────────────────────────────────────────────

// GetShipment godoc
// @Summary      Ambil detail pengiriman
// @Description  Mengambil detail pengiriman termasuk item-itemnya
// @Tags         Shipments
// @Param        id   path      string  true  "Shipment ID"
// @Produce      json
// @Success      200  {object}  utils.Response{data=models.Shipment}
// @Failure      404  {object}  utils.Response
// @Router       /shipments/{id} [get]
// @Security     BearerAuth
func GetShipment(c *fiber.Ctx) error {
	id := c.Params("id")
	shipment, err := services.GetShipmentByID(id)
	if err != nil {
		return utils.JSONError(c, fiber.StatusNotFound, "Pengiriman tidak ditemukan")
	}
	return utils.JSONSuccess(c, "Detail pengiriman berhasil diambil", shipment)
}

// ─────────────────────────────────────────────
// Create Shipment (Partial Shipment)
// ─────────────────────────────────────────────

// CreateShipment godoc
// @Summary      Buat pengiriman baru (partial shipment)
// @Description  Membuat pengiriman baru dari order. Jumlah kirim tidak boleh melebihi sisa qty. Invoice otomatis di-generate. Status order diperbarui otomatis (partial/shipped).
// @Tags         Shipments
// @Accept       multipart/form-data
// @Produce      json
// @Param        order_id      formData  string  true  "Order ID"
// @Param        shipping_date formData  string  true  "Shipping Date (YYYY-MM-DD)"
// @Param        items         formData  string  true  "JSON string of array of items e.g. [{'order_item_id':'...', 'shipping_qty':50}]"
// @Param        bukti_kirim   formData  file    true  "File Bukti Kirim"
// @Param        surat_jalan   formData  file    true  "File Surat Jalan"
// @Param        bon_pesanan   formData  file    true  "File Bon Pesanan"
// @Param        invoice_file  formData  file    false "File Invoice"
// @Success      201   {object}  utils.Response{data=models.Shipment}
// @Failure      400   {object}  utils.Response
// @Router       /shipments [post]
// @Security     BearerAuth
func CreateShipment(c *fiber.Ctx) error {
	orderID := c.FormValue("order_id")
	shippingDate := c.FormValue("shipping_date")
	itemsJSON := c.FormValue("items")

	var itemsPayload []ShipmentItemRequestPayload
	if itemsJSON != "" {
		if err := json.Unmarshal([]byte(itemsJSON), &itemsPayload); err != nil {
			return utils.JSONError(c, fiber.StatusBadRequest, "Format items tidak valid. Harus berupa array JSON.")
		}
	}

	buktiKirim, errBukti := c.FormFile("bukti_kirim")
	if errBukti != nil || buktiKirim == nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "File Bukti Kirim wajib dilampirkan")
	}

	suratJalan, errSurat := c.FormFile("surat_jalan")
	if errSurat != nil || suratJalan == nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "File Surat Jalan wajib dilampirkan")
	}

	bonPesanan, errBon := c.FormFile("bon_pesanan")
	if errBon != nil || bonPesanan == nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "File Bon Pesanan wajib dilampirkan")
	}

	userID, err := services.ParseUserID(c.Locals("userID").(string))
	if err != nil {
		return utils.JSONError(c, fiber.StatusUnauthorized, err.Error())
	}

	var items []services.ShipmentItemInput
	for _, item := range itemsPayload {
		items = append(items, services.ShipmentItemInput{
			OrderItemID: item.OrderItemID,
			ShippingQty: item.ShippingQty,
		})
	}

	shipment, err := services.CreateShipment(services.CreateShipmentInput{
		OrderID:      orderID,
		ShippingDate: shippingDate,
		Items:        items,
	}, userID)
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}

	// ─────────────────────────────────────────────
	// Handle File Uploads
	// ─────────────────────────────────────────────
	shipmentIDStr := shipment.ID.String()

	services.UploadAttachment(buktiKirim, shipmentIDStr, "shipment_delivery", userID)
	services.UploadAttachment(suratJalan, shipmentIDStr, "surat_jalan", userID)
	services.UploadAttachment(bonPesanan, shipmentIDStr, "bon", userID)

	if invoiceFile, err := c.FormFile("invoice_file"); err == nil && invoiceFile != nil {
		if invoice, err := services.GetInvoiceByShipmentID(shipmentIDStr); err == nil {
			services.UploadAttachment(invoiceFile, invoice.ID.String(), "invoice", userID)
		}
	}

	return utils.JSONCreated(c, "Pengiriman berhasil dibuat dan invoice otomatis di-generate", shipment)
}

// ─────────────────────────────────────────────
// Confirm Received
// ─────────────────────────────────────────────

// ConfirmShipmentReceived godoc
// @Summary      Konfirmasi penerimaan barang
// @Description  Mengubah status pengiriman menjadi 'diterima' dan mencatat tanggal penerimaan. Jika semua item terkirim dan semua shipment diterima, status order menjadi 'completed'.
// @Tags         Shipments
// @Param        id   path      string  true  "Shipment ID"
// @Produce      json
// @Success      200  {object}  utils.Response{data=models.Shipment}
// @Failure      400  {object}  utils.Response
// @Router       /shipments/{id}/received [patch]
// @Security     BearerAuth
func ConfirmShipmentReceived(c *fiber.Ctx) error {
	id := c.Params("id")
	userID, err := services.ParseUserID(c.Locals("userID").(string))
	if err != nil {
		return utils.JSONError(c, fiber.StatusUnauthorized, err.Error())
	}

	shipment, err := services.ConfirmShipmentReceived(id, userID)
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}

	return utils.JSONSuccess(c, "Penerimaan barang berhasil dikonfirmasi", shipment)
}
