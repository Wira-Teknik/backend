package services

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"teknik/config"
	"teknik/models"
	"teknik/utils"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Sentinel errors for Payment service
var (
	ErrPaymentInvalidUUID = errors.New("ID pembayaran tidak valid")
	ErrPaymentNotFound    = errors.New("pembayaran tidak ditemukan")
)

// ─────────────────────────────────────────────
// DTOs
// ─────────────────────────────────────────────

type CreatePaymentInput struct {
	PaymentDate  string   `json:"payment_date"` // format: "2006-01-02"
	PaymentTotal float64  `json:"payment_total"`
	OrderIDs     []string `json:"order_ids"`
}

// ─────────────────────────────────────────────
// Get All Payments
// ─────────────────────────────────────────────

type PaymentOrderResponse struct {
	ID                 uuid.UUID            `json:"id"`
	TransactionNo      string               `json:"transaction_no"`
	PoNo               string               `json:"po_no"`
	OrderDate          utils.JSONDate       `json:"order_date"`
	TotalAmountToPay   float64              `json:"total_amount_to_pay"`
	RemainingBalance   float64              `json:"remaining_balance"`
	PaymentStatus      models.PaymentStatus `json:"payment_status"`
	UnpaidInvoiceTotal float64              `json:"unpaid_invoice_total"`
	UnpaidInvoiceCount int                  `json:"unpaid_invoice_count"`
}

type CustomerPaymentSummary struct {
	CustomerName string  `json:"customer_name"`
	TotalUnpaid  int     `json:"total_unpaid"`
	TotalPaid    int     `json:"total_paid"`
	TotalPartial int     `json:"total_partial"`
	TotalTagihan float64 `json:"total_tagihan"`
}

type OrderPaymentDTO struct {
	PaymentID       uuid.UUID      `json:"payment_id"`
	PaymentDate     utils.JSONDate `json:"payment_date"`
	AllocatedAmount float64        `json:"allocated_amount"`
}

type OrderWithPaymentsDTO struct {
	ID                 uuid.UUID            `json:"id"`
	TransactionNo      string               `json:"transaction_no"`
	PoNo               string               `json:"po_no"`
	OrderDate          utils.JSONDate       `json:"order_date"`
	TotalAmountToPay   float64              `json:"total_amount_to_pay"`
	RemainingBalance   float64              `json:"remaining_balance"`
	PaymentStatus      models.PaymentStatus `json:"payment_status"`
	Invoices           []PaymentInvoiceDTO  `json:"invoices"`
	UnpaidInvoiceTotal float64              `json:"unpaid_invoice_total"`
	UnpaidInvoiceCount int                  `json:"unpaid_invoice_count"`
	Payments           []OrderPaymentDTO    `json:"payments"`
}

type PaymentInvoiceDTO struct {
	ID               uuid.UUID            `json:"id"`
	ShipmentID       uuid.UUID            `json:"shipment_id"`
	InvoiceNo        string               `json:"invoice_no"`
	TotalAmount      float64              `json:"total_amount"`
	RemainingBalance float64              `json:"remaining_balance"`
	PaymentStatus    models.PaymentStatus `json:"payment_status"`
}

type CustomerPaymentDetailResponse struct {
	CustomerName     string                 `json:"customer_name"`
	RemainingBalance float64                `json:"remaining_balance"`
	Orders           []OrderWithPaymentsDTO `json:"orders"`
}

// SearchCustomerPayments mencari customer berdasarkan nama (dari data pesanan) dan menghitung tagihannya.
func SearchCustomerPayments(name, startDate, endDate, status string, page, limit int) ([]CustomerPaymentSummary, int64, error) {
	// Validate status first
	if status != "" && !strings.EqualFold(status, "all") {
		statusLower := strings.ToLower(strings.TrimSpace(status))
		validStatuses := map[string]bool{
			"unpaid":  true,
			"partial": true,
			"paid":    true,
		}
		if !validStatuses[statusLower] {
			return nil, 0, fmt.Errorf("status pembayaran tidak valid, gunakan: all, unpaid, partial, paid")
		}
	}

	var orders []models.Order

	query := config.DB.Preload("Items").
		Preload("Shipments").
		Preload("Shipments.Items").
		Preload("Shipments.Invoice").
		Order("created_at DESC")

	if name != "" {
		query = query.Where("recipient_name ILIKE ?", "%"+name+"%")
	}

	layout := "2006-01-02"
	if startDate != "" {
		start, err := time.ParseInLocation(layout, startDate, time.Local)
		if err != nil {
			return nil, 0, fmt.Errorf("format start_date tidak valid, gunakan YYYY-MM-DD")
		}
		query = query.Where("order_date >= ?", start)
	}
	if endDate != "" {
		end, err := time.ParseInLocation(layout, endDate, time.Local)
		if err != nil {
			return nil, 0, fmt.Errorf("format end_date tidak valid, gunakan YYYY-MM-DD")
		}
		end = end.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
		query = query.Where("order_date <= ?", end)
	}

	if err := query.Find(&orders).Error; err != nil {
		return nil, 0, err
	}

	// 1. Kumpulkan semua ID Invoice dari order yang ditemukan dan inisialisasi slice Payments kosong
	var allInvoiceIDs []uuid.UUID
	invoiceToOrderMap := make(map[uuid.UUID]int) // memetakan ID Invoice ke index Order di dalam slice orders

	for i := range orders {
		orders[i].Payments = []models.Payment{} // inisialisasi slice kosong

		// Populate order.Invoices from shipments in memory
		var orderInvoices []models.Invoice
		for _, shp := range orders[i].Shipments {
			if shp.Invoice != nil {
				orderInvoices = append(orderInvoices, *shp.Invoice)
			}
		}
		orders[i].Invoices = orderInvoices

		computeOrderPaymentInfo(&orders[i])

		for _, shp := range orders[i].Shipments {
			if shp.Invoice != nil {
				allInvoiceIDs = append(allInvoiceIDs, shp.Invoice.ID)
				invoiceToOrderMap[shp.Invoice.ID] = i
			}
		}
	}

	// 2. Tarik semua Payment yang membayar invoice tersebut (Optimasi performa menghindari N+1 query)
	if len(allInvoiceIDs) > 0 {
		var payments []models.Payment
		err := config.DB.Preload("Details").
			Joins("JOIN payment_details ON payment_details.payment_id = payments.id").
			Where("payment_details.invoice_id IN ?", allInvoiceIDs).
			Group("payments.id").
			Order("payments.payment_date ASC, payments.created_at ASC").
			Find(&payments).Error

		if err != nil {
			return nil, 0, err
		}

		for _, p := range payments {
			addedToOrder := make(map[int]bool)
			for _, detail := range p.Details {
				if orderIdx, exists := invoiceToOrderMap[detail.InvoiceID]; exists {
					if !addedToOrder[orderIdx] {
						orders[orderIdx].Payments = append(orders[orderIdx].Payments, p)
						addedToOrder[orderIdx] = true
					}
				}
			}
		}
	}

	// 3. Group by recipient_name and preserve order of newest orders
	customerMap := make(map[string]*CustomerPaymentSummary)
	var orderedCustomerNames []string

	for i := range orders {
		// Filter by payment status if specified
		if status != "" && !strings.EqualFold(status, "all") {
			statusLower := strings.ToLower(strings.TrimSpace(status))
			if !strings.EqualFold(string(orders[i].PaymentStatus), statusLower) {
				continue
			}
		}

		custName := orders[i].RecipientName
		if _, exists := customerMap[custName]; !exists {
			customerMap[custName] = &CustomerPaymentSummary{
				CustomerName: custName,
				TotalUnpaid:  0,
				TotalPaid:    0,
				TotalPartial: 0,
				TotalTagihan: 0,
			}
			orderedCustomerNames = append(orderedCustomerNames, custName)
		}

		switch orders[i].PaymentStatus {
		case models.PaymentStatusUnpaid:
			customerMap[custName].TotalUnpaid++
		case models.PaymentStatusPaid:
			customerMap[custName].TotalPaid++
		case models.PaymentStatusPartial:
			customerMap[custName].TotalPartial++
		}

		customerMap[custName].TotalTagihan += orders[i].RemainingBalance
	}

	var results []CustomerPaymentSummary
	results = make([]CustomerPaymentSummary, 0, len(orderedCustomerNames))
	for _, custName := range orderedCustomerNames {
		summary := customerMap[custName]
		summary.TotalTagihan = roundTwo(summary.TotalTagihan)
		results = append(results, *summary)
	}

	totalRows := int64(len(results))

	// In-memory pagination
	if page > 0 && limit > 0 {
		start := (page - 1) * limit
		if start >= len(results) {
			results = []CustomerPaymentSummary{}
		} else {
			end := start + limit
			if end > len(results) {
				end = len(results)
			}
			results = results[start:end]
		}
	}

	return results, totalRows, nil
}

func GetAllPayments() ([]models.Payment, error) {
	var payments []models.Payment
	err := config.DB.Preload("Details").
		Order("created_at DESC").
		Find(&payments).Error
	return payments, err
}

// ─────────────────────────────────────────────
// Get Payment By ID
// ─────────────────────────────────────────────

func GetPaymentByID(id string) (models.Payment, error) {
	if _, err := uuid.Parse(id); err != nil {
		return models.Payment{}, ErrPaymentInvalidUUID
	}

	var payment models.Payment
	err := config.DB.Preload("Details").
		First(&payment, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.Payment{}, ErrPaymentNotFound
		}
		return models.Payment{}, err
	}
	return payment, nil
}

// ─────────────────────────────────────────────
// Create Payment (Lunas / Cicilan / Kolektif)
// ─────────────────────────────────────────────
// Alur:
// 1. Validasi setiap detail: invoice harus ada dan belum lunas
// 2. allocated_amount <= remaining_balance
// 3. Buat Payment + PaymentDetails dalam transaksi
// 4. Update remaining_balance & payment_status per invoice
// 5. payment_total = sum(allocated_amount)

func CreatePayment(input CreatePaymentInput, userID uuid.UUID) (models.Payment, error) {
	if len(input.OrderIDs) == 0 {
		return models.Payment{}, fmt.Errorf("pembayaran harus memiliki minimal 1 order ID")
	}
	if input.PaymentTotal <= 0 {
		return models.Payment{}, fmt.Errorf("total pembayaran harus lebih dari 0")
	}

	paymentDate, err := time.ParseInLocation("2006-01-02", input.PaymentDate, time.Local)
	if err != nil {
		return models.Payment{}, fmt.Errorf("format tanggal tidak valid (gunakan YYYY-MM-DD)")
	}

	// Validasi dan parsing semua OrderIDs ke UUID untuk mencegah PostgreSQL parsing crash
	var parsedOrderIDs []uuid.UUID
	for _, idStr := range input.OrderIDs {
		parsedID, err := uuid.Parse(idStr)
		if err != nil {
			return models.Payment{}, fmt.Errorf("ID order %s tidak valid", idStr)
		}
		parsedOrderIDs = append(parsedOrderIDs, parsedID)
	}

	// 1. Ambil semua invoice terkait order_ids yang belum lunas
	var invoices []models.Invoice

	err = config.DB.
		Joins("JOIN shipments ON shipments.id = invoices.shipment_id").
		Where("shipments.order_id IN ? AND invoices.payment_status != ?", parsedOrderIDs, models.PaymentStatusPaid).
		Order("invoices.created_at ASC"). // urutkan yang paling lama dahulu
		Find(&invoices).Error

	if err != nil {
		return models.Payment{}, fmt.Errorf("gagal mengambil tagihan: %w", err)
	}

	if len(invoices) == 0 {
		return models.Payment{}, fmt.Errorf("tidak ada tagihan yang belum lunas untuk order yang dipilih")
	}

	// 2. Validasi total payment <= total remaining balance
	var totalTagihan float64
	for _, inv := range invoices {
		totalTagihan += inv.RemainingBalance
	}

	if input.PaymentTotal > totalTagihan {
		return models.Payment{}, fmt.Errorf("jumlah pembayaran (%.2f) melebihi total sisa tagihan (%.2f)", input.PaymentTotal, totalTagihan)
	}

	// 3. Mulai alokasi dana secara otomatis
	paymentID := uuid.New()
	var details []models.PaymentDetail

	type invoiceAlloc struct {
		invoice         models.Invoice
		allocatedAmount float64
	}
	var allocations []invoiceAlloc

	sisaPembayaran := input.PaymentTotal

	for _, inv := range invoices {
		if sisaPembayaran <= 0 {
			break
		}

		alokasi := sisaPembayaran
		if alokasi > inv.RemainingBalance {
			alokasi = inv.RemainingBalance
		}

		details = append(details, models.PaymentDetail{
			ID:              uuid.New(),
			PaymentID:       paymentID,
			InvoiceID:       inv.ID,
			AllocatedAmount: alokasi,
		})

		allocations = append(allocations, invoiceAlloc{
			invoice:         inv,
			allocatedAmount: alokasi,
		})

		sisaPembayaran -= alokasi
		sisaPembayaran = roundTwo(sisaPembayaran)
	}

	// 4. Mulai transaksi database
	tx := config.DB.Begin()

	payment := models.Payment{
		ID:           paymentID,
		PaymentTotal: input.PaymentTotal,
		PaymentDate:  utils.JSONDate(paymentDate),
		Details:      details,
	}

	if err := tx.Create(&payment).Error; err != nil {
		tx.Rollback()
		return models.Payment{}, fmt.Errorf("gagal membuat pembayaran: %w", err)
	}

	// Update setiap invoice
	for _, alloc := range allocations {
		newBalance := roundTwo(alloc.invoice.RemainingBalance - alloc.allocatedAmount)

		var newStatus models.PaymentStatus
		if newBalance <= 0 {
			newStatus = models.PaymentStatusPaid
			newBalance = 0
		} else {
			newStatus = models.PaymentStatusPartial
		}

		if err := tx.Model(&models.Invoice{}).
			Where("id = ?", alloc.invoice.ID).
			Updates(map[string]interface{}{
				"remaining_balance": newBalance,
				"payment_status":    newStatus,
			}).Error; err != nil {
			tx.Rollback()
			return models.Payment{}, fmt.Errorf("gagal memperbarui invoice: %w", err)
		}
	}

	if err := tx.Commit().Error; err != nil {
		return models.Payment{}, fmt.Errorf("gagal menyimpan data pembayaran: %w", err)
	}

	CreateAuditLog(userID, paymentID, models.AuditActionCreate, "payments", nil, payment)

	return payment, nil
}

// UpdatePaymentTotal memperbarui payment_total pembayaran yang ada dan mengalokasikan ulang secara atomik.
func UpdatePaymentTotal(paymentIDStr string, newTotal float64, orderIDs []string, userID uuid.UUID) (models.Payment, error) {
	paymentID, err := uuid.Parse(paymentIDStr)
	if err != nil {
		return models.Payment{}, ErrPaymentInvalidUUID
	}

	if newTotal <= 0 {
		return models.Payment{}, fmt.Errorf("total pembayaran baru harus lebih dari 0")
	}

	// Validasi dan parsing semua OrderIDs ke UUID untuk mencegah PostgreSQL parsing crash
	var parsedOrderIDs []uuid.UUID
	for _, idStr := range orderIDs {
		parsedID, err := uuid.Parse(idStr)
		if err != nil {
			return models.Payment{}, fmt.Errorf("ID order %s tidak valid", idStr)
		}
		parsedOrderIDs = append(parsedOrderIDs, parsedID)
	}

	// 1. Mulai transaksi database
	tx := config.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 2. Ambil data payment beserta details lama
	var payment models.Payment
	if err := tx.Preload("Details").First(&payment, "id = ?", paymentID).Error; err != nil {
		tx.Rollback()
		return models.Payment{}, fmt.Errorf("pembayaran tidak ditemukan: %w", err)
	}

	// Buat salinan data payment lama untuk audit log
	oldPaymentData := payment

	// 3. REVERT: Kembalikan alokasi lama ke masing-masing Invoice
	invoiceAllocations := make(map[uuid.UUID]float64)
	var oldInvoiceIDs []uuid.UUID
	for _, detail := range payment.Details {
		invoiceAllocations[detail.InvoiceID] += detail.AllocatedAmount
		oldInvoiceIDs = append(oldInvoiceIDs, detail.InvoiceID)
	}

	if len(oldInvoiceIDs) > 0 {
		// Ambil data invoice lama yang terdampak
		var oldInvoices []models.Invoice
		if err := tx.Where("id IN ?", oldInvoiceIDs).Find(&oldInvoices).Error; err != nil {
			tx.Rollback()
			return models.Payment{}, fmt.Errorf("gagal mengambil data invoice terkait: %w", err)
		}

		// Buat map invoice lama untuk mempermudah update
		oldInvoiceMap := make(map[uuid.UUID]*models.Invoice)
		for i := range oldInvoices {
			oldInvoiceMap[oldInvoices[i].ID] = &oldInvoices[i]
		}

		// Kembalikan saldo dan status masing-masing invoice lama
		for invID, oldAlloc := range invoiceAllocations {
			inv, exists := oldInvoiceMap[invID]
			if !exists {
				tx.Rollback()
				return models.Payment{}, fmt.Errorf("invoice %s tidak ditemukan dalam database", invID)
			}

			inv.RemainingBalance = roundTwo(inv.RemainingBalance + oldAlloc)
			if inv.RemainingBalance >= inv.TotalAmount {
				inv.PaymentStatus = models.PaymentStatusUnpaid
				inv.RemainingBalance = inv.TotalAmount
			} else if inv.RemainingBalance > 0 {
				inv.PaymentStatus = models.PaymentStatusPartial
			} else {
				inv.PaymentStatus = models.PaymentStatusPaid
			}

			if err := tx.Save(inv).Error; err != nil {
				tx.Rollback()
				return models.Payment{}, fmt.Errorf("gagal mengembalikan saldo invoice %s: %w", inv.ID, err)
			}
		}
	}

	// 4. Ambil semua invoice terkait order_ids baru yang belum lunas (dari transaksi tx saat ini)
	var invoices []models.Invoice
	err = tx.
		Joins("JOIN shipments ON shipments.id = invoices.shipment_id").
		Where("shipments.order_id IN ? AND invoices.payment_status != ?", parsedOrderIDs, models.PaymentStatusPaid).
		Order("invoices.created_at ASC"). // urutkan yang paling lama dahulu
		Find(&invoices).Error

	if err != nil {
		tx.Rollback()
		return models.Payment{}, fmt.Errorf("gagal mengambil tagihan baru: %w", err)
	}

	if len(invoices) == 0 {
		tx.Rollback()
		return models.Payment{}, fmt.Errorf("tidak ada tagihan yang belum lunas untuk order yang dipilih")
	}

	// 5. Validasi total payment <= total remaining balance baru
	var totalTagihan float64
	for _, inv := range invoices {
		totalTagihan += inv.RemainingBalance
	}

	if newTotal > totalTagihan {
		tx.Rollback()
		return models.Payment{}, fmt.Errorf("jumlah pembayaran baru (%.2f) melebihi total sisa tagihan yang tersedia (%.2f)", newTotal, totalTagihan)
	}

	// 6. Hapus detail pembayaran lama
	if err := tx.Where("payment_id = ?", payment.ID).Delete(&models.PaymentDetail{}).Error; err != nil {
		tx.Rollback()
		return models.Payment{}, fmt.Errorf("gagal menghapus detail alokasi lama: %w", err)
	}

	// 7. REALLOCATE: Alokasikan newTotal secara kronologis ke invoice-invoice baru
	var newDetails []models.PaymentDetail
	sisaDana := newTotal

	for i := range invoices {
		inv := &invoices[i]
		if sisaDana <= 0 {
			break
		}

		alokasi := sisaDana
		if alokasi > inv.RemainingBalance {
			alokasi = inv.RemainingBalance
		}

		// Update invoice dengan alokasi baru
		inv.RemainingBalance = roundTwo(inv.RemainingBalance - alokasi)
		if inv.RemainingBalance <= 0 {
			inv.PaymentStatus = models.PaymentStatusPaid
			inv.RemainingBalance = 0
		} else {
			inv.PaymentStatus = models.PaymentStatusPartial
		}

		if err := tx.Save(inv).Error; err != nil {
			tx.Rollback()
			return models.Payment{}, fmt.Errorf("gagal memperbarui alokasi saldo invoice %s: %w", inv.ID, err)
		}

		// Buat detail alokasi baru
		newDetail := models.PaymentDetail{
			ID:              uuid.New(),
			PaymentID:       payment.ID,
			InvoiceID:       inv.ID,
			AllocatedAmount: alokasi,
		}

		if err := tx.Create(&newDetail).Error; err != nil {
			tx.Rollback()
			return models.Payment{}, fmt.Errorf("gagal menyimpan detail alokasi baru: %w", err)
		}

		newDetails = append(newDetails, newDetail)
		sisaDana = roundTwo(sisaDana - alokasi)
	}

	// 8. Update Payment utama
	payment.PaymentTotal = newTotal
	payment.Details = newDetails
	if err := tx.Save(&payment).Error; err != nil {
		tx.Rollback()
		return models.Payment{}, fmt.Errorf("gagal memperbarui pembayaran utama: %w", err)
	}

	// Commit transaksi
	if err := tx.Commit().Error; err != nil {
		return models.Payment{}, fmt.Errorf("gagal melakukan commit transaksi: %w", err)
	}

	// 9. Catat Audit Log UPDATE
	CreateAuditLog(userID, payment.ID, models.AuditActionUpdate, "payments", oldPaymentData, payment)

	return payment, nil
}

// GetCustomerPaymentDetail menarik ringkasan keuangan dan riwayat pembayaran lengkap dari satu customer tertentu berdasarkan namanya dengan filter opsional.
func GetCustomerPaymentDetail(customerName string, poNoFilter string, statusFilter string, startDate string, endDate string) (CustomerPaymentDetailResponse, error) {
	var orders []models.Order

	// 1. Ambil seluruh pesanan untuk customerName tersebut (case-insensitive)
	err := config.DB.Preload("Items").
		Preload("Shipments").
		Preload("Shipments.Items").
		Preload("Shipments.Invoice").
		Where("LOWER(recipient_name) = LOWER(?)", customerName).
		Order("order_date DESC, created_at DESC").
		Find(&orders).Error

	if err != nil {
		return CustomerPaymentDetailResponse{}, err
	}

	if len(orders) == 0 {
		return CustomerPaymentDetailResponse{}, fmt.Errorf("customer tidak ditemukan atau tidak memiliki riwayat pesanan")
	}

	// 2. Hitung status finansial untuk seluruh order dan hitung akumulasi saldo asli customer
	var grandRemainingBalance float64
	for i := range orders {
		var orderInvoices []models.Invoice
		for _, shp := range orders[i].Shipments {
			if shp.Invoice != nil {
				orderInvoices = append(orderInvoices, *shp.Invoice)
			}
		}
		orders[i].Invoices = orderInvoices

		computeOrderPaymentInfo(&orders[i])
		grandRemainingBalance += orders[i].RemainingBalance
	}

	// 3. Terapkan filter Nomor PO / Transaksi, Status Pembayaran, dan Rentang Tanggal di memori Go
	var filteredOrders []models.Order

	var startLimit time.Time
	var endLimit time.Time
	var hasStart, hasEnd bool

	layout := "2006-01-02"
	if startDate != "" {
		t, err := time.ParseInLocation(layout, startDate, time.Local)
		if err != nil {
			return CustomerPaymentDetailResponse{}, fmt.Errorf("format start_date tidak valid, gunakan YYYY-MM-DD")
		}
		startLimit = t
		hasStart = true
	}
	if endDate != "" {
		t, err := time.ParseInLocation(layout, endDate, time.Local)
		if err != nil {
			return CustomerPaymentDetailResponse{}, fmt.Errorf("format end_date tidak valid, gunakan YYYY-MM-DD")
		}
		endLimit = t.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
		hasEnd = true
	}

	for i := range orders {
		// Filter rentang tanggal (order_date)
		orderTime := time.Time(orders[i].OrderDate)
		if hasStart && orderTime.Before(startLimit) {
			continue
		}
		if hasEnd && orderTime.After(endLimit) {
			continue
		}

		// Filter PO No / Transaction No (case-insensitive)
		if poNoFilter != "" {
			poMatch := strings.Contains(strings.ToLower(orders[i].PoNo), strings.ToLower(poNoFilter))
			trxMatch := strings.Contains(strings.ToLower(orders[i].TransactionNo), strings.ToLower(poNoFilter))
			if !poMatch && !trxMatch {
				continue
			}
		}

		// Filter Status Pembayaran (case-insensitive, skip jika status == "all" atau kosong)
		if statusFilter != "" && !strings.EqualFold(statusFilter, "all") {
			if !strings.EqualFold(string(orders[i].PaymentStatus), statusFilter) {
				continue
			}
		}

		filteredOrders = append(filteredOrders, orders[i])
	}

	// 4. Susun DTO hasil untuk pesanan yang lolos filter
	var invoiceIDs []uuid.UUID
	invoiceToOrderIndexMap := make(map[uuid.UUID]int) // memetakan ID Invoice ke index Order di dalam slice filteredOrders
	orderDTOs := make([]OrderWithPaymentsDTO, len(filteredOrders))

	for i := range filteredOrders {
		var orderInvoicesDTO []PaymentInvoiceDTO
		for _, inv := range filteredOrders[i].Invoices {
			orderInvoicesDTO = append(orderInvoicesDTO, PaymentInvoiceDTO{
				ID:               inv.ID,
				ShipmentID:       inv.ShipmentID,
				InvoiceNo:        inv.InvoiceNo,
				TotalAmount:      inv.TotalAmount,
				RemainingBalance: inv.RemainingBalance,
				PaymentStatus:    inv.PaymentStatus,
			})
		}
		if orderInvoicesDTO == nil {
			orderInvoicesDTO = []PaymentInvoiceDTO{}
		}

		var unpaidInvoiceTotal float64
		var unpaidInvoiceCount int
		for _, inv := range orderInvoicesDTO {
			if inv.PaymentStatus == models.PaymentStatusUnpaid || inv.PaymentStatus == models.PaymentStatusPartial {
				unpaidInvoiceTotal += inv.RemainingBalance
				unpaidInvoiceCount++
			}
		}

		orderDTOs[i] = OrderWithPaymentsDTO{
			ID:                 filteredOrders[i].ID,
			TransactionNo:      filteredOrders[i].TransactionNo,
			PoNo:               filteredOrders[i].PoNo,
			OrderDate:          filteredOrders[i].OrderDate,
			TotalAmountToPay:   filteredOrders[i].TotalAmountToPay,
			RemainingBalance:   filteredOrders[i].RemainingBalance,
			PaymentStatus:      filteredOrders[i].PaymentStatus,
			Invoices:           orderInvoicesDTO,
			UnpaidInvoiceTotal: roundTwo(unpaidInvoiceTotal),
			UnpaidInvoiceCount: unpaidInvoiceCount,
			Payments:           []OrderPaymentDTO{},
		}

		for _, shp := range filteredOrders[i].Shipments {
			if shp.Invoice != nil {
				invoiceIDs = append(invoiceIDs, shp.Invoice.ID)
				invoiceToOrderIndexMap[shp.Invoice.ID] = i
			}
		}
	}

	// 5. Tarik semua Payment yang membayar invoice pesanan terfilter (Single query)
	if len(invoiceIDs) > 0 {
		var payments []models.Payment
		err := config.DB.Preload("Details").
			Joins("JOIN payment_details ON payment_details.payment_id = payments.id").
			Where("payment_details.invoice_id IN ?", invoiceIDs).
			Group("payments.id").
			Order("payments.payment_date ASC, payments.created_at ASC").
			Find(&payments).Error

		if err != nil {
			return CustomerPaymentDetailResponse{}, err
		}

		for _, p := range payments {
			for _, detail := range p.Details {
				if orderIdx, exists := invoiceToOrderIndexMap[detail.InvoiceID]; exists {
					// Tambah ke riwayat pembayaran order yang sesuai
					orderDTOs[orderIdx].Payments = append(orderDTOs[orderIdx].Payments, OrderPaymentDTO{
						PaymentID:       p.ID,
						PaymentDate:     p.PaymentDate,
						AllocatedAmount: detail.AllocatedAmount,
					})
				}
			}
		}
	}

	return CustomerPaymentDetailResponse{
		CustomerName:     customerName,
		RemainingBalance: roundTwo(grandRemainingBalance),
		Orders:           orderDTOs,
	}, nil
}

// ─────────────────────────────────────────────
// Payment History Report
// ─────────────────────────────────────────────

type PaymentHistoryDTO struct {
	ID            string  `json:"id" example:"550e8400-e29b-41d4-a716-446655440001"`
	TransactionNo string  `json:"transaction_no" example:"NF/WT/1/2026"`
	PoNo          string  `json:"po_no" example:"P2152KPT22"`
	CustomerName  string  `json:"customer_name" example:"PT.KIRANA PERMATA"`
	AdminName     string  `json:"admin_name" example:"Admin - Dino"`
	TotalAmount   float64 `json:"total_amount" example:"500000"`
	PaymentStatus string  `json:"payment_status" example:"Partial"`
	CreatedAt     string  `json:"created_at" example:"2026-11-05 12:10"`
}

func GetPaymentHistory(search string, statusFilter string, startDate string, endDate string, page int, limit int) ([]PaymentHistoryDTO, int64, error) {
	type rawPaymentHistory struct {
		DetailID        uuid.UUID
		PaymentID       uuid.UUID
		TransactionNo   string
		PoNo            string
		CustomerName    string
		AllocatedAmount float64
		OrderID         uuid.UUID
		CreatedAt       time.Time
	}

	var raws []rawPaymentHistory
	query := config.DB.Table("payment_details").
		Select("payment_details.id AS detail_id, payment_details.payment_id, orders.transaction_no, orders.po_no, orders.recipient_name AS customer_name, payments.created_at AS created_at, payment_details.allocated_amount, orders.id AS order_id").
		Joins("JOIN payments ON payments.id = payment_details.payment_id").
		Joins("JOIN invoices ON invoices.id = payment_details.invoice_id").
		Joins("JOIN shipments ON shipments.id = invoices.shipment_id").
		Joins("JOIN orders ON orders.id = shipments.order_id").
		Where("payment_details.deleted_at IS NULL").
		Where("payments.deleted_at IS NULL").
		Where("invoices.deleted_at IS NULL").
		Where("shipments.deleted_at IS NULL").
		Where("orders.deleted_at IS NULL")

	if search != "" {
		searchTerm := "%" + search + "%"
		query = query.Where("orders.transaction_no ILIKE ? OR orders.po_no ILIKE ?", searchTerm, searchTerm)
	}

	layout := "2006-01-02"
	if startDate != "" {
		start, err := time.ParseInLocation(layout, startDate, time.Local)
		if err != nil {
			return nil, 0, fmt.Errorf("format start_date tidak valid, gunakan YYYY-MM-DD")
		}
		query = query.Where("payments.payment_date >= ?", start)
	}
	if endDate != "" {
		end, err := time.ParseInLocation(layout, endDate, time.Local)
		if err != nil {
			return nil, 0, fmt.Errorf("format end_date tidak valid, gunakan YYYY-MM-DD")
		}
		end = end.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
		query = query.Where("payments.payment_date <= ?", end)
	}

	err := query.Order("payments.created_at DESC, payment_details.created_at DESC").Scan(&raws).Error
	if err != nil {
		return nil, 0, err
	}

	if len(raws) == 0 {
		return []PaymentHistoryDTO{}, 0, nil
	}

	// 1. Kumpulkan semua ID Order unik untuk ditarik relasinya dalam satu batch query
	var orderIDs []uuid.UUID
	orderIDMap := make(map[uuid.UUID]bool)
	for _, r := range raws {
		if !orderIDMap[r.OrderID] {
			orderIDMap[r.OrderID] = true
			orderIDs = append(orderIDs, r.OrderID)
		}
	}

	var orders []models.Order
	if len(orderIDs) > 0 {
		err := config.DB.Preload("Items").
			Preload("Shipments").
			Preload("Shipments.Items").
			Preload("Shipments.Invoice").
			Where("id IN ?", orderIDs).
			Find(&orders).Error
		if err != nil {
			return nil, 0, err
		}
	}

	ordersMap := make(map[uuid.UUID]models.Order)
	for i := range orders {
		var allInvoices []models.Invoice
		for _, shp := range orders[i].Shipments {
			if shp.Invoice != nil {
				allInvoices = append(allInvoices, *shp.Invoice)
			}
		}
		orders[i].Invoices = allInvoices

		computeOrderPaymentInfo(&orders[i])
		ordersMap[orders[i].ID] = orders[i]
	}

	// 2. Kumpulkan ID Payment unik untuk mengambil data admin pembuatnya dari audit logs
	var paymentIDs []uuid.UUID
	paymentIDMap := make(map[uuid.UUID]bool)
	for _, r := range raws {
		if !paymentIDMap[r.PaymentID] {
			paymentIDMap[r.PaymentID] = true
			paymentIDs = append(paymentIDs, r.PaymentID)
		}
	}

	type auditUser struct {
		ResourceID uuid.UUID
		AdminName  string
	}
	var auditUsers []auditUser
	if len(paymentIDs) > 0 {
		if err := config.DB.Table("audit_logs").
			Select("audit_logs.resource_id, users.name as admin_name").
			Joins("JOIN users ON users.id = audit_logs.user_id AND users.deleted_at IS NULL").
			Where("audit_logs.action = ? AND audit_logs.table_name = ? AND audit_logs.resource_id IN ?", models.AuditActionCreate, "payments", paymentIDs).
			Scan(&auditUsers).Error; err != nil {
			return nil, 0, err
		}
	}

	adminMap := make(map[uuid.UUID]string)
	for _, au := range auditUsers {
		adminMap[au.ResourceID] = au.AdminName
	}

	// 3. Bangun DTO hasil akhir dengan filter status pembayaran
	var results []PaymentHistoryDTO
	statusFilterLower := strings.ToLower(strings.TrimSpace(statusFilter))

	for _, r := range raws {
		order, exists := ordersMap[r.OrderID]
		if !exists {
			continue
		}

		if statusFilterLower != "" && statusFilterLower != "all" {
			if strings.ToLower(string(order.PaymentStatus)) != statusFilterLower {
				continue
			}
		}

		adminName := adminMap[r.PaymentID]
		if adminName == "" {
			adminName = "Unknown"
		} else {
			adminName = "Admin - " + strings.ToUpper(adminName)
		}

		createdAtFormatted := r.CreatedAt.Local().Format("2006-01-02 15:04")

		statusFormatted := "Unpaid"
		switch order.PaymentStatus {
		case models.PaymentStatusPaid:
			statusFormatted = "Paid"
		case models.PaymentStatusPartial:
			statusFormatted = "Partial"
		}

		results = append(results, PaymentHistoryDTO{
			ID:            r.DetailID.String(),
			TransactionNo: r.TransactionNo,
			PoNo:          r.PoNo,
			CustomerName:  r.CustomerName,
			AdminName:     adminName,
			CreatedAt:     createdAtFormatted,
			TotalAmount:   r.AllocatedAmount,
			PaymentStatus: statusFormatted,
		})
	}

	totalRows := int64(len(results))

	if limit > 0 && page > 0 {
		offset := (page - 1) * limit
		if offset >= len(results) {
			results = []PaymentHistoryDTO{}
		} else {
			end := offset + limit
			if end > len(results) {
				end = len(results)
			}
			results = results[offset:end]
		}
	}

	if results == nil {
		results = []PaymentHistoryDTO{}
	}

	return results, totalRows, nil
}
