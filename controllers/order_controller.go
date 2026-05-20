package controllers

import (
	"teknik/services"
	"teknik/utils"

	"github.com/gofiber/fiber/v2"
)

// ─────────────────────────────────────────────
// Request DTOs (exported for Swagger)
// ─────────────────────────────────────────────

// CreateOrderRequest adalah body untuk membuat pesanan baru.
type CreateOrderRequest struct {
	TransactionNo    string                    `json:"transaction_no"     example:"NF/WT/1/2026"`
	PoNo             string                    `json:"po_no"              example:"PO-2026-001"`
	OrderDate        string                    `json:"order_date"         example:"2026-05-09"`
	RecipientName    string                    `json:"recipient_name"     example:"PT Maju Jaya"`
	RecipientAddress string                    `json:"recipient_address"  example:"Jl. Industri No. 10"`
	RecipientPhone   string                    `json:"recipient_phone"    example:"081234567890"`
	RecipientEmail   string                    `json:"recipient_email"    example:"purchasing@majujaya.com"`
	Items            []OrderItemRequestPayload `json:"items"`
}

// OrderItemRequestPayload adalah detail item dalam pesanan.
type OrderItemRequestPayload struct {
	ProductName string  `json:"product_name" example:"Bearing SKF 6205"`
	OrderQty    int     `json:"order_qty"    example:"100"`
	UnitPrice   float64 `json:"unit_price"   example:"75000"`
}

// UpdateOrderRequest adalah body untuk mengupdate header pesanan.
type UpdateOrderRequest struct {
	PoNo             string `json:"po_no"              example:"PO-2026-001"`
	RecipientName    string `json:"recipient_name"     example:"PT Maju Jaya"`
	RecipientAddress string `json:"recipient_address"  example:"Jl. Industri No. 10"`
	RecipientPhone   string `json:"recipient_phone"    example:"081234567890"`
	RecipientEmail   string `json:"recipient_email"    example:"purchasing@majujaya.com"`
}

// ─────────────────────────────────────────────
// Get All Orders
// ─────────────────────────────────────────────

// GetAllOrders godoc
// @Summary      Ambil semua pesanan
// @Description  Mengambil daftar semua pesanan beserta item-itemnya
// @Tags         Orders
// @Produce      json
// @Success      200  {object}  utils.Response{data=[]models.Order}
// @Router       /orders [get]
// @Security     BearerAuth
func GetAllOrders(c *fiber.Ctx) error {
	orders, err := services.GetAllOrders()
	if err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, "Gagal mengambil data pesanan")
	}
	return utils.JSONSuccess(c, "Data pesanan berhasil diambil", orders)
}

// ─────────────────────────────────────────────
// Transaction History Report
// ─────────────────────────────────────────────

// GetTransactionHistory godoc
// @Summary      Ambil Laporan Riwayat Transaksi
// @Description  Mengambil riwayat transaksi dengan pencarian berdasarkan PO atau nomor transaksi dan filter berdasarkan status pembayaran (all, paid, partial)
// @Tags         Orders
// @Produce      json
// @Param        search query string false "Cari nomor PO atau transaksi"
// @Param        status query string false "Filter status (all, paid, partial)"
// @Success      200  {object}  utils.Response{data=[]services.TransactionHistoryDTO}
// @Router       /orders/history [get]
// @Security     BearerAuth
func GetTransactionHistory(c *fiber.Ctx) error {
	search := c.Query("search")
	status := c.Query("status", "all")

	history, err := services.GetTransactionHistory(search, status)
	if err != nil {
		return utils.JSONError(c, fiber.StatusInternalServerError, "Gagal mengambil riwayat transaksi: "+err.Error())
	}
	return utils.JSONSuccess(c, "Riwayat transaksi berhasil diambil", history)
}

// ─────────────────────────────────────────────
// Get Order by ID
// ─────────────────────────────────────────────

// GetOrder godoc
// @Summary      Ambil detail pesanan
// @Description  Mengambil detail pesanan termasuk items dan shipments
// @Tags         Orders
// @Param        id   path      string  true  "Order ID"
// @Produce      json
// @Success      200  {object}  utils.Response{data=models.Order}
// @Failure      404  {object}  utils.Response
// @Router       /orders/{id} [get]
// @Security     BearerAuth
func GetOrder(c *fiber.Ctx) error {
	id := c.Params("id")
	order, err := services.GetOrderByID(id)
	if err != nil {
		return utils.JSONError(c, fiber.StatusNotFound, "Pesanan tidak ditemukan")
	}
	return utils.JSONSuccess(c, "Detail pesanan berhasil diambil", order)
}

// ─────────────────────────────────────────────
// Get Next Transaction No
// ─────────────────────────────────────────────

// GetNextTransactionNo godoc
// @Summary      Pratinjau nomor transaksi berikutnya
// @Description  Mengambil format nomor transaksi yang akan digunakan untuk pesanan baru
// @Tags         Orders
// @Produce      json
// @Success      200  {object}  utils.Response{data=string}
// @Router       /orders/next-trx [get]
// @Security     BearerAuth
func GetNextTransactionNo(c *fiber.Ctx) error {
	nextTrx := services.GenerateTransactionNo()
	return utils.JSONSuccess(c, "Nomor transaksi berikutnya berhasil diambil", map[string]string{
		"transaction_no": nextTrx,
	})
}

// ─────────────────────────────────────────────
// Create Order
// ─────────────────────────────────────────────

// CreateOrder godoc
// @Summary      Buat pesanan baru
// @Description  Membuat pesanan baru dengan item. PPN 11% dan subtotal dihitung otomatis. remaining_qty diset sama dengan order_qty.
// @Tags         Orders
// @Accept       json
// @Produce      json
// @Param        body  body      CreateOrderRequest  true  "Data pesanan"
// @Success      201   {object}  utils.Response{data=models.Order}
// @Failure      400   {object}  utils.Response
// @Router       /orders [post]
// @Security     BearerAuth
func CreateOrder(c *fiber.Ctx) error {
	var req CreateOrderRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "Format request tidak valid")
	}

	userID, err := services.ParseUserID(c.Locals("userID").(string))
	if err != nil {
		return utils.JSONError(c, fiber.StatusUnauthorized, err.Error())
	}

	// Convert request items
	var items []services.OrderItemInput
	for _, item := range req.Items {
		items = append(items, services.OrderItemInput{
			ProductName: item.ProductName,
			OrderQty:    item.OrderQty,
			UnitPrice:   item.UnitPrice,
		})
	}

	order, err := services.CreateOrder(services.CreateOrderInput{
		TransactionNo:    req.TransactionNo,
		PoNo:             req.PoNo,
		OrderDate:        req.OrderDate,
		RecipientName:    req.RecipientName,
		RecipientAddress: req.RecipientAddress,
		RecipientPhone:   req.RecipientPhone,
		RecipientEmail:   req.RecipientEmail,
		Items:            items,
	}, userID)
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}

	return utils.JSONCreated(c, "Pesanan berhasil dibuat", order)
}

// ─────────────────────────────────────────────
// Update Order
// ─────────────────────────────────────────────

// UpdateOrder godoc
// @Summary      Update header pesanan
// @Description  Mengupdate data header pesanan (hanya jika status masih pending)
// @Tags         Orders
// @Accept       json
// @Produce      json
// @Param        id    path      string              true  "Order ID"
// @Param        body  body      UpdateOrderRequest   true  "Data pesanan"
// @Success      200   {object}  utils.Response{data=models.Order}
// @Failure      400   {object}  utils.Response
// @Router       /orders/{id} [put]
// @Security     BearerAuth
func UpdateOrder(c *fiber.Ctx) error {
	id := c.Params("id")
	var req UpdateOrderRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, "Format request tidak valid")
	}

	userID, err := services.ParseUserID(c.Locals("userID").(string))
	if err != nil {
		return utils.JSONError(c, fiber.StatusUnauthorized, err.Error())
	}

	order, err := services.UpdateOrder(id, services.UpdateOrderInput{
		PoNo:             req.PoNo,
		RecipientName:    req.RecipientName,
		RecipientAddress: req.RecipientAddress,
		RecipientPhone:   req.RecipientPhone,
		RecipientEmail:   req.RecipientEmail,
	}, userID)
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}

	return utils.JSONSuccess(c, "Pesanan berhasil diupdate", order)
}

// ─────────────────────────────────────────────
// Delete Order
// ─────────────────────────────────────────────

// DeleteOrder godoc
// @Summary      Hapus pesanan
// @Description  Menghapus pesanan beserta item-itemnya (hanya jika status masih pending)
// @Tags         Orders
// @Param        id   path      string  true  "Order ID"
// @Produce      json
// @Success      200  {object}  utils.Response
// @Failure      400  {object}  utils.Response
// @Router       /orders/{id} [delete]
// @Security     BearerAuth
func DeleteOrder(c *fiber.Ctx) error {
	id := c.Params("id")
	userID, err := services.ParseUserID(c.Locals("userID").(string))
	if err != nil {
		return utils.JSONError(c, fiber.StatusUnauthorized, err.Error())
	}

	if err := services.DeleteOrder(id, userID); err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}

	return utils.JSONSuccess(c, "Pesanan berhasil dihapus", nil)
}
