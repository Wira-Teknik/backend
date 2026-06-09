package services

import (
	"fmt"
	"strings"
	"time"

	"teknik/config"
	"teknik/models"
)

// ─────────────────────────────────────────────
// DTOs
// ─────────────────────────────────────────────

type RecapFilter struct {
	StartDate string
	EndDate   string
	Search    string
	Status    string
	Page      int
	Limit     int
}

type RecapSummaryDTO struct {
	TotalPendapatan   float64 `json:"total_pendapatan" example:"45200000"`
	TotalPesananCount int64   `json:"total_pesanan_count" example:"142"`
	TotalUnpaid       float64 `json:"total_unpaid" example:"12800000"`
	TotalPaid         float64 `json:"total_paid" example:"32400000"`
	LunasPercentage   float64 `json:"lunas_percentage" example:"71.6"`
}

type RecapInvoiceItemDTO struct {
	TransactionNo string  `json:"transaction_no" example:"T10001"`
	PoNo          string  `json:"po_no" example:"P2152KPT22"`
	CustomerName  string  `json:"customer_name" example:"PT.KIRANA PERMATA"`
	Date          string  `json:"date" example:"Oct 21, 2026"`
	TotalAmount   float64 `json:"total_amount" example:"1120000"`
	PaymentStatus string  `json:"payment_status" example:"Unpaid"`
}

type RecapOrderItemDTO struct {
	TransactionNo string  `json:"transaction_no" example:"T10001"`
	PoNo         string  `json:"po_no" example:"P5537XJJ22"`
	CustomerName string  `json:"customer_name" example:"PT.KIRANA PERMATA"`
	Date         string  `json:"date" example:"Oct 23, 2026"`
	TotalAmount  float64 `json:"total_amount" example:"2000000"`
	OrderStatus  string  `json:"order_status" example:"Pending"`
}

type DetailPendapatanDTO struct {
	TotalPendapatan   float64               `json:"total_pendapatan" example:"45200000"`
	TotalPesananCount int64                 `json:"total_pesanan_count" example:"142"`
	TotalItemCount    int64                 `json:"total_item_count" example:"98"`
	Items             []RecapInvoiceItemDTO `json:"items"`
}

type DetailPesananDTO struct {
	TotalPesananAmount float64             `json:"total_pesanan_amount" example:"45200000"`
	TotalPesananCount  int64               `json:"total_pesanan_count" example:"142"`
	Items              []RecapOrderItemDTO `json:"items"`
}

type DetailUnpaidDTO struct {
	TotalUnpaid       float64               `json:"total_unpaid" example:"12800000"`
	TotalCount        int64                 `json:"total_count" example:"8"`
	TotalPesananCount int64                 `json:"total_pesanan_count" example:"142"`
	Items             []RecapInvoiceItemDTO `json:"items"`
}

type DetailPaidDTO struct {
	TotalPaid         float64               `json:"total_paid" example:"32400000"`
	TotalCount        int64                 `json:"total_count" example:"134"`
	TotalPesananCount int64                 `json:"total_pesanan_count" example:"142"`
	Items             []RecapInvoiceItemDTO `json:"items"`
}

// ─────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────

func parseRecapDates(f RecapFilter) (time.Time, time.Time, error) {
	layout := "2006-01-02"
	start := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Now().Add(24 * time.Hour)
	var err error

	if f.StartDate != "" {
		start, err = time.Parse(layout, f.StartDate)
		if err != nil {
			return start, end, fmt.Errorf("format start_date tidak valid, gunakan YYYY-MM-DD")
		}
	}
	if f.EndDate != "" {
		end, err = time.Parse(layout, f.EndDate)
		if err != nil {
			return start, end, fmt.Errorf("format end_date tidak valid, gunakan YYYY-MM-DD")
		}
		end = end.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
	}
	return start, end, nil
}

type invoiceRaw struct {
	TransactionNo    string
	PoNo             string
	CustomerName     string
	OrderDate        time.Time
	TotalAmount      float64
	RemainingBalance float64
	PaymentStatus    string
}

// queryInvoices mengambil SEMUA invoice (tanpa pagination) – dipakai untuk kalkulasi agregat.
func queryInvoices(start, end time.Time, search string, statusFilter ...models.PaymentStatus) ([]invoiceRaw, error) {
	var rows []invoiceRaw
	q := config.DB.Table("invoices").
		Select("orders.transaction_no, orders.po_no, orders.recipient_name AS customer_name, orders.order_date, " +
			"invoices.total_amount, invoices.remaining_balance, invoices.payment_status").
		Joins("JOIN shipments ON shipments.id = invoices.shipment_id AND shipments.deleted_at IS NULL").
		Joins("JOIN orders ON orders.id = shipments.order_id AND orders.deleted_at IS NULL").
		Where("orders.order_date >= ? AND orders.order_date <= ?", start, end).
		Where("invoices.deleted_at IS NULL")

	if search != "" {
		s := "%" + strings.ToLower(search) + "%"
		q = q.Where("LOWER(orders.recipient_name) LIKE ? OR LOWER(orders.po_no) LIKE ?", s, s)
	}
	if len(statusFilter) > 0 {
		q = q.Where("invoices.payment_status IN ?", statusFilter)
	}

	err := q.Order("orders.order_date DESC").Scan(&rows).Error
	return rows, err
}

// queryInvoicesPage mengambil invoice dengan LIMIT/OFFSET (pagination di DB) dan mengembalikan total count.
func queryInvoicesPage(start, end time.Time, search string, page, limit int, statusFilter ...models.PaymentStatus) ([]invoiceRaw, int64, error) {
	base := config.DB.Table("invoices").
		Joins("JOIN shipments ON shipments.id = invoices.shipment_id AND shipments.deleted_at IS NULL").
		Joins("JOIN orders ON orders.id = shipments.order_id AND orders.deleted_at IS NULL").
		Where("orders.order_date >= ? AND orders.order_date <= ?", start, end).
		Where("invoices.deleted_at IS NULL")

	if search != "" {
		s := "%" + strings.ToLower(search) + "%"
		base = base.Where("LOWER(orders.recipient_name) LIKE ? OR LOWER(orders.po_no) LIKE ?", s, s)
	}
	if len(statusFilter) > 0 {
		base = base.Where("invoices.payment_status IN ?", statusFilter)
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rows []invoiceRaw
	q := base.
		Select("orders.transaction_no, orders.po_no, orders.recipient_name AS customer_name, orders.order_date, " +
			"invoices.total_amount, invoices.remaining_balance, invoices.payment_status").
		Order("orders.order_date DESC")

	if limit > 0 {
		offset := (page - 1) * limit
		q = q.Limit(limit).Offset(offset)
	}

	err := q.Scan(&rows).Error
	return rows, total, err
}

func mapPaymentStatus(s string) string {
	switch models.PaymentStatus(s) {
	case models.PaymentStatusPaid:
		return "Paid"
	case models.PaymentStatusPartial:
		return "Partial"
	default:
		return "Unpaid"
	}
}

func mapOrderStatus(s string) string {
	switch models.OrderStatus(s) {
	case models.OrderStatusCompleted:
		return "Completed"
	case models.OrderStatusShipped:
		return "Shipped"
	case models.OrderStatusPartial:
		return "Partial"
	default:
		return "Pending"
	}
}

// ─────────────────────────────────────────────
// 1. Summary
// ─────────────────────────────────────────────

func GetPaymentRecapSummary(f RecapFilter) (RecapSummaryDTO, error) {
	var resp RecapSummaryDTO

	start, end, err := parseRecapDates(f)
	if err != nil {
		return resp, err
	}

	rows, err := queryInvoices(start, end, f.Search)
	if err != nil {
		return resp, err
	}

	for _, r := range rows {
		resp.TotalPendapatan += r.TotalAmount
		if r.PaymentStatus == string(models.PaymentStatusUnpaid) || r.PaymentStatus == string(models.PaymentStatusPartial) {
			resp.TotalUnpaid += r.RemainingBalance
		}
	}
	resp.TotalPaid = roundTwo(resp.TotalPendapatan - resp.TotalUnpaid)
	resp.TotalPendapatan = roundTwo(resp.TotalPendapatan)
	resp.TotalUnpaid = roundTwo(resp.TotalUnpaid)

	// Count orders in period
	orderQ := config.DB.Model(&models.Order{}).Where("order_date >= ? AND order_date <= ?", start, end)
	if f.Search != "" {
		s := "%" + strings.ToLower(f.Search) + "%"
		orderQ = orderQ.Where("LOWER(recipient_name) LIKE ? OR LOWER(po_no) LIKE ?", s, s)
	}
	if err := orderQ.Count(&resp.TotalPesananCount).Error; err != nil {
		return resp, err
	}

	if resp.TotalPendapatan > 0 {
		resp.LunasPercentage = roundTwo((resp.TotalPaid / resp.TotalPendapatan) * 100)
	}

	return resp, nil
}

// ─────────────────────────────────────────────
// 2. Detail Pendapatan
// ─────────────────────────────────────────────

func GetDetailPendapatan(f RecapFilter) (DetailPendapatanDTO, int64, error) {
	var resp DetailPendapatanDTO

	start, end, err := parseRecapDates(f)
	if err != nil {
		return resp, 0, err
	}

	// 1. Hitung Total Pendapatan Asli (berdasarkan Tanggal dan Pencarian, tanpa filter Status)
	unfilteredRows, err := queryInvoices(start, end, f.Search)
	if err != nil {
		return resp, 0, err
	}
	for _, r := range unfilteredRows {
		resp.TotalPendapatan += r.TotalAmount
	}
	resp.TotalPendapatan = roundTwo(resp.TotalPendapatan)

	// 2. Hitung Total Pesanan (berdasarkan Tanggal dan Pencarian, tanpa filter Status)
	orderQ := config.DB.Model(&models.Order{}).Where("order_date >= ? AND order_date <= ?", start, end).Where("deleted_at IS NULL")
	if f.Search != "" {
		s := "%" + strings.ToLower(f.Search) + "%"
		orderQ = orderQ.Where("LOWER(recipient_name) LIKE ? OR LOWER(po_no) LIKE ?", s, s)
	}
	if err := orderQ.Count(&resp.TotalPesananCount).Error; err != nil {
		return resp, 0, err
	}

	// 3. Tarik data Invoice yang sudah difilter berdasarkan Status untuk list item (dengan pagination)
	var statusFilter []models.PaymentStatus
	switch strings.ToLower(f.Status) {
	case "paid":
		statusFilter = []models.PaymentStatus{models.PaymentStatusPaid}
	case "unpaid":
		statusFilter = []models.PaymentStatus{models.PaymentStatusUnpaid, models.PaymentStatusPartial}
	}

	page := f.Page
	if page < 1 {
		page = 1
	}

	rows, totalItems, err := queryInvoicesPage(start, end, f.Search, page, f.Limit, statusFilter...)
	if err != nil {
		return resp, 0, err
	}
	resp.TotalItemCount = totalItems

	resp.Items = []RecapInvoiceItemDTO{}
	for _, r := range rows {
		resp.Items = append(resp.Items, RecapInvoiceItemDTO{
			TransactionNo: r.TransactionNo,
			PoNo:          r.PoNo,
			CustomerName:  r.CustomerName,
			Date:          r.OrderDate.Local().Format("2006-01-02"),
			TotalAmount:   r.TotalAmount,
			PaymentStatus: mapPaymentStatus(r.PaymentStatus),
		})
	}

	return resp, totalItems, nil
}

// ─────────────────────────────────────────────
// 3. Detail Pesanan
// ─────────────────────────────────────────────

func GetDetailPesanan(f RecapFilter) (DetailPesananDTO, int64, error) {
	var resp DetailPesananDTO

	start, end, err := parseRecapDates(f)
	if err != nil {
		return resp, 0, err
	}

	type orderRaw struct {
		TransactionNo string
		PoNo          string
		CustomerName  string
		OrderDate     time.Time
		OrderStatus   string
		TotalAmount   float64
	}

	// 1. Hitung Total Pesanan & Nominal Kotor untuk Header (tanpa filter Status)
	overallQ := config.DB.Table("orders").
		Select("orders.transaction_no, orders.po_no, orders.recipient_name AS customer_name, orders.order_date, "+
			"orders.order_status, COALESCE(SUM(items.subtotal), 0) AS total_amount").
		Joins("LEFT JOIN order_items items ON items.order_id = orders.id AND items.deleted_at IS NULL").
		Where("orders.order_date >= ? AND orders.order_date <= ?", start, end).
		Where("orders.deleted_at IS NULL").
		Group("orders.id, orders.transaction_no, orders.po_no, orders.recipient_name, orders.order_date, orders.order_status")

	if f.Search != "" {
		s := "%" + strings.ToLower(f.Search) + "%"
		overallQ = overallQ.Where("LOWER(orders.recipient_name) LIKE ? OR LOWER(orders.po_no) LIKE ?", s, s)
	}

	var overallRows []orderRaw
	if err := overallQ.Scan(&overallRows).Error; err != nil {
		return resp, 0, err
	}

	resp.TotalPesananCount = int64(len(overallRows))
	for _, r := range overallRows {
		resp.TotalPesananAmount += r.TotalAmount
	}
	resp.TotalPesananAmount = roundTwo(resp.TotalPesananAmount)

	// 2. Tarik daftar pesanan terfilter status untuk List Item (dengan pagination di DB)
	statusLower := strings.ToLower(f.Status)

	// Hitung total item yang akan dipaginasi (menggunakan count distinct)
	var totalItems int64
	countDistQ := config.DB.Table("orders").
		Joins("LEFT JOIN order_items items ON items.order_id = orders.id AND items.deleted_at IS NULL").
		Where("orders.order_date >= ? AND orders.order_date <= ?", start, end).
		Where("orders.deleted_at IS NULL")
	if f.Search != "" {
		s := "%" + strings.ToLower(f.Search) + "%"
		countDistQ = countDistQ.Where("LOWER(orders.recipient_name) LIKE ? OR LOWER(orders.po_no) LIKE ?", s, s)
	}
	if statusLower != "" && statusLower != "all" {
		countDistQ = countDistQ.Where("orders.order_status = ?", statusLower)
	}
	if err := countDistQ.Distinct("orders.id").Count(&totalItems).Error; err != nil {
		return resp, 0, err
	}

	q := config.DB.Table("orders").
		Select("orders.transaction_no, orders.po_no, orders.recipient_name AS customer_name, orders.order_date, "+
			"orders.order_status, COALESCE(SUM(items.subtotal), 0) AS total_amount").
		Joins("LEFT JOIN order_items items ON items.order_id = orders.id AND items.deleted_at IS NULL").
		Where("orders.order_date >= ? AND orders.order_date <= ?", start, end).
		Where("orders.deleted_at IS NULL").
		Group("orders.id, orders.transaction_no, orders.po_no, orders.recipient_name, orders.order_date, orders.order_status")

	if f.Search != "" {
		s := "%" + strings.ToLower(f.Search) + "%"
		q = q.Where("LOWER(orders.recipient_name) LIKE ? OR LOWER(orders.po_no) LIKE ?", s, s)
	}
	if statusLower != "" && statusLower != "all" {
		q = q.Where("orders.order_status = ?", statusLower)
	}

	page := f.Page
	if page < 1 {
		page = 1
	}
	q = q.Order("orders.order_date DESC")
	if f.Limit > 0 {
		offset := (page - 1) * f.Limit
		q = q.Limit(f.Limit).Offset(offset)
	}

	var rows []orderRaw
	if err := q.Scan(&rows).Error; err != nil {
		return resp, 0, err
	}

	resp.Items = []RecapOrderItemDTO{}
	for _, r := range rows {
		resp.Items = append(resp.Items, RecapOrderItemDTO{
			TransactionNo: r.TransactionNo,
			PoNo:          r.PoNo,
			CustomerName:  r.CustomerName,
			Date:          r.OrderDate.Local().Format("2006-01-02"),
			TotalAmount:   r.TotalAmount,
			OrderStatus:   mapOrderStatus(r.OrderStatus),
		})
	}

	return resp, totalItems, nil
}

// ─────────────────────────────────────────────
// 4. Detail Unpaid
// ─────────────────────────────────────────────

func GetDetailUnpaid(f RecapFilter) (DetailUnpaidDTO, int64, error) {
	var resp DetailUnpaidDTO

	start, end, err := parseRecapDates(f)
	if err != nil {
		return resp, 0, err
	}

	// 1. Hitung Total Pesanan (berdasarkan Tanggal dan Pencarian) untuk Header Badge
	orderQ := config.DB.Model(&models.Order{}).Where("order_date >= ? AND order_date <= ?", start, end).Where("deleted_at IS NULL")
	if f.Search != "" {
		s := "%" + strings.ToLower(f.Search) + "%"
		orderQ = orderQ.Where("LOWER(recipient_name) LIKE ? OR LOWER(po_no) LIKE ?", s, s)
	}
	if err := orderQ.Count(&resp.TotalPesananCount).Error; err != nil {
		return resp, 0, err
	}

	// 2. Hitung total invoice Unpaid & Partial (untuk header aggregat)
	allRows, err := queryInvoices(start, end, f.Search, models.PaymentStatusUnpaid, models.PaymentStatusPartial)
	if err != nil {
		return resp, 0, err
	}
	resp.TotalCount = int64(len(allRows))
	for _, r := range allRows {
		resp.TotalUnpaid += r.RemainingBalance
	}
	resp.TotalUnpaid = roundTwo(resp.TotalUnpaid)

	// 3. Tarik item dengan pagination di DB
	page := f.Page
	if page < 1 {
		page = 1
	}
	rows, _, err := queryInvoicesPage(start, end, f.Search, page, f.Limit, models.PaymentStatusUnpaid, models.PaymentStatusPartial)
	if err != nil {
		return resp, 0, err
	}

	resp.Items = []RecapInvoiceItemDTO{}
	for _, r := range rows {
		resp.Items = append(resp.Items, RecapInvoiceItemDTO{
			TransactionNo: r.TransactionNo,
			PoNo:          r.PoNo,
			CustomerName:  r.CustomerName,
			Date:          r.OrderDate.Local().Format("2006-01-02"),
			TotalAmount:   r.RemainingBalance,
			PaymentStatus: mapPaymentStatus(r.PaymentStatus),
		})
	}
	return resp, resp.TotalCount, nil
}

// ─────────────────────────────────────────────
// 5. Detail Paid
// ─────────────────────────────────────────────

func GetDetailPaid(f RecapFilter) (DetailPaidDTO, int64, error) {
	var resp DetailPaidDTO

	start, end, err := parseRecapDates(f)
	if err != nil {
		return resp, 0, err
	}

	// 1. Hitung Total Pesanan (berdasarkan Tanggal dan Pencarian) untuk Header Badge
	orderQ := config.DB.Model(&models.Order{}).Where("order_date >= ? AND order_date <= ?", start, end).Where("deleted_at IS NULL")
	if f.Search != "" {
		s := "%" + strings.ToLower(f.Search) + "%"
		orderQ = orderQ.Where("LOWER(recipient_name) LIKE ? OR LOWER(po_no) LIKE ?", s, s)
	}
	if err := orderQ.Count(&resp.TotalPesananCount).Error; err != nil {
		return resp, 0, err
	}

	// 2. Hitung total invoice Paid (untuk header agregat)
	allRows, err := queryInvoices(start, end, f.Search, models.PaymentStatusPaid)
	if err != nil {
		return resp, 0, err
	}
	resp.TotalCount = int64(len(allRows))
	for _, r := range allRows {
		resp.TotalPaid += r.TotalAmount
	}
	resp.TotalPaid = roundTwo(resp.TotalPaid)

	// 3. Tarik item dengan pagination di DB
	page := f.Page
	if page < 1 {
		page = 1
	}
	rows, _, err := queryInvoicesPage(start, end, f.Search, page, f.Limit, models.PaymentStatusPaid)
	if err != nil {
		return resp, 0, err
	}

	resp.Items = []RecapInvoiceItemDTO{}
	for _, r := range rows {
		resp.Items = append(resp.Items, RecapInvoiceItemDTO{
			TransactionNo: r.TransactionNo,
			PoNo:          r.PoNo,
			CustomerName:  r.CustomerName,
			Date:          r.OrderDate.Local().Format("2006-01-02"),
			TotalAmount:   r.TotalAmount,
			PaymentStatus: "Paid",
		})
	}
	return resp, resp.TotalCount, nil
}
