package controllers

import (
	"errors"
	"strconv"

	"teknik/models"
	"teknik/services"
	"teknik/utils"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
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
	OrderDate        string `json:"order_date"         example:"2026-05-13"`
	RecipientName    string `json:"recipient_name"     example:"PT Maju Jaya"`
	RecipientAddress string `json:"recipient_address"  example:"Jl. Industri No. 10"`
	RecipientPhone   string `json:"recipient_phone"    example:"081234567890"`
	RecipientEmail   string `json:"recipient_email"    example:"purchasing@majujaya.com"`
}

// OrderListItemResponse adalah data item pesanan ringkas yang dikembalikan dalam list pesanan.
type OrderListItemResponse struct {
	ID               string         `json:"id"                  example:"6ec26fbf-a256-4a67-a89f-2136eff97857"`
	TransactionNo    string         `json:"transaction_no"      example:"NF/WT/100/2026"`
	PoNo             string         `json:"po_no"               example:"561/PO-WLK/XI/26"`
	OrderDate        utils.JSONDate `json:"order_date"          example:"2026-11-02"`
	RecipientName    string         `json:"recipient_name"      example:"PT Sentosa Abadi"`
	OrderStatus      string         `json:"order_status"        example:"pending"`
	TotalAmountToPay float64        `json:"total_amount_to_pay" example:"7492500"`
}

// OrderItemResponse adalah detail item dalam pesanan (tanpa created_at & updated_at).
type OrderItemResponse struct {
	ID           uuid.UUID `json:"id"`
	OrderID      uuid.UUID `json:"order_id"`
	ProductName  string    `json:"product_name"`
	OrderQty     int       `json:"order_qty"`
	RemainingQty int       `json:"remaining_qty"`
	UnitPrice    float64   `json:"unit_price"`
	PPN          float64   `json:"ppn"`
	Subtotal     float64   `json:"subtotal"`
}

// ShipmentItemResponse adalah detail item dalam pengiriman (tanpa created_at & updated_at).
type ShipmentItemResponse struct {
	ID          uuid.UUID `json:"id"`
	ShipmentID  uuid.UUID `json:"shipment_id"`
	OrderItemID uuid.UUID `json:"order_item_id"`
	ShippingQty int       `json:"shipping_qty"`
}

// InvoiceResponse adalah detail invoice (tanpa created_at & updated_at).
type InvoiceResponse struct {
	ID               uuid.UUID            `json:"id"`
	ShipmentID       uuid.UUID            `json:"shipment_id"`
	InvoiceNo        string               `json:"invoice_no"`
	TotalAmount      float64              `json:"total_amount"`
	RemainingBalance float64              `json:"remaining_balance"`
	PaymentStatus    models.PaymentStatus `json:"payment_status"`
}

// ShipmentResponse adalah detail pengiriman (tanpa created_at & updated_at).
type ShipmentResponse struct {
	ID             uuid.UUID              `json:"id"`
	OrderID        uuid.UUID              `json:"order_id"`
	ShippingDate   utils.JSONDate         `json:"shipping_date"`
	ReceivedDate   *utils.JSONDate        `json:"received_date"`
	ShippingStatus models.ShippingStatus  `json:"shipping_status"`
	Items          []ShipmentItemResponse `json:"items"`
}

// OrderDetailResponse adalah data detail pesanan lengkap (tanpa created_at & updated_at di level root, items, dan shipments).
type OrderDetailResponse struct {
	ID               uuid.UUID            `json:"id"`
	TransactionNo    string               `json:"transaction_no"`
	PoNo             string               `json:"po_no"`
	OrderDate        utils.JSONDate       `json:"order_date"`
	RecipientName    string               `json:"recipient_name"`
	RecipientAddress string               `json:"recipient_address"`
	RecipientPhone   string               `json:"recipient_phone"`
	RecipientEmail   string               `json:"recipient_email"`
	OrderStatus      models.OrderStatus   `json:"order_status"`
	Items            []OrderItemResponse  `json:"items"`
	Shipments        []ShipmentResponse   `json:"shipments"`
	Invoices         []InvoiceResponse    `json:"invoices"`
	TotalAmountToPay float64              `json:"total_amount_to_pay"`
	RemainingBalance float64              `json:"remaining_balance"`
	PaymentStatus    models.PaymentStatus `json:"payment_status"`
	Payments         []models.Payment     `json:"payments"`
	UpdatedAt        utils.JSONDateTime   `json:"updated_at"`
}

// PaginationMeta adalah metadata pagination untuk API response.
type PaginationMeta struct {
	Page       int   `json:"page"       example:"1"`
	Limit      int   `json:"limit"      example:"20"`
	TotalRows  int64 `json:"total_rows" example:"100"`
	TotalPages int   `json:"total_pages" example:"5"`
}

// PaginatedOrdersResponse adalah wrapper response paginasi untuk daftar pesanan.
type PaginatedOrdersResponse struct {
	Success    bool                    `json:"success"    example:"true"`
	Message    string                  `json:"message"    example:"Data pesanan berhasil diambil"`
	Data       []OrderListItemResponse `json:"data"`
	Pagination PaginationMeta          `json:"pagination"`
}

// ─────────────────────────────────────────────
// Get All Orders
// ─────────────────────────────────────────────

// GetAllOrders godoc
// @Summary      Ambil semua pesanan
// @Description  Mengambil daftar semua pesanan beserta item-itemnya. Mendukung pencarian, filter rentang tanggal order_date, status pesanan (order_status), dan pagination (page & limit).
// @Tags         Orders
// @Produce      json
// @Param        start_date      query  string  false  "Tanggal awal filter (YYYY-MM-DD)"
// @Param        end_date        query  string  false  "Tanggal akhir filter (YYYY-MM-DD)"
// @Param        search          query  string  false  "Cari nomor PO, nomor transaksi, atau nama perusahaan"
// @Param        order_status    query  string  false  "Status order (all, pending, partial, shipped, completed)"
// @Param        page            query  int     false  "Halaman aktif (default: 1)"
// @Param        limit           query  int     false  "Jumlah baris per halaman (default: 20)"
// @Success      200  {object}  controllers.PaginatedOrdersResponse
// @Router       /orders [get]
// @Security     BearerAuth
func GetAllOrders(c *fiber.Ctx) error {
	startDate := c.Query("start_date")
	endDate := c.Query("end_date")
	search := c.Query("search")
	orderStatus := c.Query("order_status")

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

	orders, totalRows, err := services.GetAllOrders(startDate, endDate, search, orderStatus, page, limit)
	if err != nil {
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}

	response := make([]OrderListItemResponse, len(orders))
	for i, o := range orders {
		response[i] = OrderListItemResponse{
			ID:               o.ID.String(),
			TransactionNo:    o.TransactionNo,
			PoNo:             o.PoNo,
			OrderDate:        o.OrderDate,
			RecipientName:    o.RecipientName,
			OrderStatus:      string(o.OrderStatus),
			TotalAmountToPay: o.TotalAmountToPay,
		}
	}

	totalPages := 0
	if totalRows > 0 {
		totalPages = int((totalRows + int64(limit) - 1) / int64(limit))
	}

	return c.Status(fiber.StatusOK).JSON(PaginatedOrdersResponse{
		Success: true,
		Message: "Data pesanan berhasil diambil",
		Data:    response,
		Pagination: PaginationMeta{
			Page:       page,
			Limit:      limit,
			TotalRows:  totalRows,
			TotalPages: totalPages,
		},
	})
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
// @Success      200  {object}  utils.Response{data=controllers.OrderDetailResponse}
// @Failure      440  {object}  utils.Response
// @Router       /orders/{id} [get]
// @Security     BearerAuth
func GetOrder(c *fiber.Ctx) error {
	id := c.Params("id")
	order, err := services.GetOrderByID(id)
	if err != nil {
		if errors.Is(err, services.ErrOrderInvalidUUID) {
			return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
		}
		if errors.Is(err, services.ErrOrderNotFound) {
			return utils.JSONError(c, fiber.StatusNotFound, err.Error())
		}
		return utils.JSONError(c, fiber.StatusInternalServerError, "Gagal mengambil detail pesanan")
	}

	// Map items
	itemsRes := make([]OrderItemResponse, len(order.Items))
	for i, item := range order.Items {
		itemsRes[i] = OrderItemResponse{
			ID:           item.ID,
			OrderID:      item.OrderID,
			ProductName:  item.ProductName,
			OrderQty:     item.OrderQty,
			RemainingQty: item.RemainingQty,
			UnitPrice:    item.UnitPrice,
			PPN:          item.PPN,
			Subtotal:     item.Subtotal,
		}
	}

	// Map shipments
	shipmentsRes := make([]ShipmentResponse, len(order.Shipments))
	for i, shp := range order.Shipments {
		// Map shipment items
		shpItemsRes := make([]ShipmentItemResponse, len(shp.Items))
		for j, shpItem := range shp.Items {
			shpItemsRes[j] = ShipmentItemResponse{
				ID:          shpItem.ID,
				ShipmentID:  shpItem.ShipmentID,
				OrderItemID: shpItem.OrderItemID,
				ShippingQty: shpItem.ShippingQty,
			}
		}

		shipmentsRes[i] = ShipmentResponse{
			ID:             shp.ID,
			OrderID:        shp.OrderID,
			ShippingDate:   shp.ShippingDate,
			ReceivedDate:   shp.ReceivedDate,
			ShippingStatus: shp.ShippingStatus,
			Items:          shpItemsRes,
		}
	}

	// Map order invoices
	invoicesRes := make([]InvoiceResponse, len(order.Invoices))
	for i, inv := range order.Invoices {
		invoicesRes[i] = InvoiceResponse{
			ID:               inv.ID,
			ShipmentID:       inv.ShipmentID,
			InvoiceNo:        inv.InvoiceNo,
			TotalAmount:      inv.TotalAmount,
			RemainingBalance: inv.RemainingBalance,
			PaymentStatus:    inv.PaymentStatus,
		}
	}

	response := OrderDetailResponse{
		ID:               order.ID,
		TransactionNo:    order.TransactionNo,
		PoNo:             order.PoNo,
		OrderDate:        order.OrderDate,
		RecipientName:    order.RecipientName,
		RecipientAddress: order.RecipientAddress,
		RecipientPhone:   order.RecipientPhone,
		RecipientEmail:   order.RecipientEmail,
		OrderStatus:      order.OrderStatus,
		Items:            itemsRes,
		Shipments:        shipmentsRes,
		Invoices:         invoicesRes,
		Payments:         order.Payments,
		TotalAmountToPay: order.TotalAmountToPay,
		RemainingBalance: order.RemainingBalance,
		PaymentStatus:    order.PaymentStatus,
		UpdatedAt:        order.UpdatedAt,
	}

	if response.Items == nil {
		response.Items = []OrderItemResponse{}
	}
	if response.Shipments == nil {
		response.Shipments = []ShipmentResponse{}
	}
	if response.Invoices == nil {
		response.Invoices = []InvoiceResponse{}
	}
	if response.Payments == nil {
		response.Payments = []models.Payment{}
	}

	return utils.JSONSuccess(c, "Detail pesanan berhasil diambil", response)
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
		if errors.Is(err, services.ErrOrderDuplicateTransactionNo) {
			return utils.JSONError(c, fiber.StatusConflict, err.Error()) // 409 Conflict
		}
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
		OrderDate:        req.OrderDate,
		RecipientName:    req.RecipientName,
		RecipientAddress: req.RecipientAddress,
		RecipientPhone:   req.RecipientPhone,
		RecipientEmail:   req.RecipientEmail,
	}, userID)
	if err != nil {
		if errors.Is(err, services.ErrOrderInvalidUUID) {
			return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
		}
		if errors.Is(err, services.ErrOrderNotFound) {
			return utils.JSONError(c, fiber.StatusNotFound, err.Error())
		}
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
		if errors.Is(err, services.ErrOrderInvalidUUID) {
			return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
		}
		if errors.Is(err, services.ErrOrderNotFound) {
			return utils.JSONError(c, fiber.StatusNotFound, err.Error())
		}
		return utils.JSONError(c, fiber.StatusBadRequest, err.Error())
	}

	return utils.JSONSuccess(c, "Pesanan berhasil dihapus", nil)
}
