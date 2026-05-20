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
}

type RecapSummaryDTO struct {
	TotalPendapatan   float64 `json:"total_pendapatan" example:"45200000"`
	TotalPesananCount int64   `json:"total_pesanan_count" example:"142"`
	TotalUnpaid       float64 `json:"total_unpaid" example:"12800000"`
	TotalPaid         float64 `json:"total_paid" example:"32400000"`
	LunasPercentage   float64 `json:"lunas_percentage" example:"71.6"`
}

type RecapInvoiceItemDTO struct {
	PoNo          string  `json:"po_no" example:"P2152KPT22"`
	CustomerName  string  `json:"customer_name" example:"PT.KIRANA PERMATA"`
	Date          string  `json:"date" example:"Oct 21, 2026"`
	TotalAmount   float64 `json:"total_amount" example:"1120000"`
	PaymentStatus string  `json:"payment_status" example:"Unpaid"`
}

type RecapOrderItemDTO struct {
	PoNo         string  `json:"po_no" example:"P5537XJJ22"`
	CustomerName string  `json:"customer_name" example:"PT.KIRANA PERMATA"`
	Date         string  `json:"date" example:"Oct 23, 2026"`
	TotalAmount  float64 `json:"total_amount" example:"2000000"`
	OrderStatus  string  `json:"order_status" example:"Pending"`
}

type DetailPendapatanDTO struct {
	TotalPendapatan   float64               `json:"total_pendapatan" example:"45200000"`
	TotalPesananCount int64                 `json:"total_pesanan_count" example:"142"`
	Items             []RecapInvoiceItemDTO `json:"items"`
}

type DetailPesananDTO struct {
	TotalPesananAmount float64             `json:"total_pesanan_amount" example:"45200000"`
	TotalPesananCount  int64               `json:"total_pesanan_count" example:"142"`
	Items              []RecapOrderItemDTO `json:"items"`
}

type DetailUnpaidDTO struct {
	TotalUnpaid float64               `json:"total_unpaid" example:"12800000"`
	TotalCount  int64                 `json:"total_count" example:"8"`
	Items       []RecapInvoiceItemDTO `json:"items"`
}

type DetailPaidDTO struct {
	TotalPaid  float64               `json:"total_paid" example:"32400000"`
	TotalCount int64                 `json:"total_count" example:"134"`
	Items      []RecapInvoiceItemDTO `json:"items"`
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
	PoNo             string
	CustomerName     string
	OrderDate        time.Time
	TotalAmount      float64
	RemainingBalance float64
	PaymentStatus    string
}

func queryInvoices(start, end time.Time, search string, statusFilter ...models.PaymentStatus) ([]invoiceRaw, error) {
	var rows []invoiceRaw
	q := config.DB.Table("invoices").
		Select("orders.po_no, orders.recipient_name AS customer_name, orders.order_date, " +
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
	orderQ.Count(&resp.TotalPesananCount)

	if resp.TotalPendapatan > 0 {
		resp.LunasPercentage = roundTwo((resp.TotalPaid / resp.TotalPendapatan) * 100)
	}

	return resp, nil
}

// ─────────────────────────────────────────────
// 2. Detail Pendapatan
// ─────────────────────────────────────────────

func GetDetailPendapatan(f RecapFilter) (DetailPendapatanDTO, error) {
	var resp DetailPendapatanDTO

	start, end, err := parseRecapDates(f)
	if err != nil {
		return resp, err
	}

	var statusFilter []models.PaymentStatus
	switch strings.ToLower(f.Status) {
	case "paid":
		statusFilter = []models.PaymentStatus{models.PaymentStatusPaid}
	case "unpaid":
		statusFilter = []models.PaymentStatus{models.PaymentStatusUnpaid, models.PaymentStatusPartial}
	}

	rows, err := queryInvoices(start, end, f.Search, statusFilter...)
	if err != nil {
		return resp, err
	}

	// Count total orders (unfiltered by status for summary)
	config.DB.Model(&models.Order{}).Where("order_date >= ? AND order_date <= ?", start, end).Count(&resp.TotalPesananCount)

	for _, r := range rows {
		resp.TotalPendapatan += r.TotalAmount
		resp.Items = append(resp.Items, RecapInvoiceItemDTO{
			PoNo:          r.PoNo,
			CustomerName:  r.CustomerName,
			Date:          r.OrderDate.Local().Format("Jan 02, 2006"),
			TotalAmount:   r.TotalAmount,
			PaymentStatus: mapPaymentStatus(r.PaymentStatus),
		})
	}
	resp.TotalPendapatan = roundTwo(resp.TotalPendapatan)
	if resp.Items == nil {
		resp.Items = []RecapInvoiceItemDTO{}
	}
	return resp, nil
}

// ─────────────────────────────────────────────
// 3. Detail Pesanan
// ─────────────────────────────────────────────

func GetDetailPesanan(f RecapFilter) (DetailPesananDTO, error) {
	var resp DetailPesananDTO

	start, end, err := parseRecapDates(f)
	if err != nil {
		return resp, err
	}

	type orderRaw struct {
		PoNo        string
		CustomerName string
		OrderDate   time.Time
		OrderStatus string
		TotalAmount float64
	}

	q := config.DB.Table("orders").
		Select("orders.po_no, orders.recipient_name AS customer_name, orders.order_date, "+
			"orders.order_status, COALESCE(SUM(items.subtotal), 0) AS total_amount").
		Joins("LEFT JOIN order_items items ON items.order_id = orders.id AND items.deleted_at IS NULL").
		Where("orders.order_date >= ? AND orders.order_date <= ?", start, end).
		Where("orders.deleted_at IS NULL").
		Group("orders.id, orders.po_no, orders.recipient_name, orders.order_date, orders.order_status")

	if f.Search != "" {
		s := "%" + strings.ToLower(f.Search) + "%"
		q = q.Where("LOWER(orders.recipient_name) LIKE ? OR LOWER(orders.po_no) LIKE ?", s, s)
	}
	statusLower := strings.ToLower(f.Status)
	if statusLower != "" && statusLower != "all" {
		q = q.Where("orders.order_status = ?", statusLower)
	}

	var rows []orderRaw
	if err := q.Order("orders.order_date DESC").Scan(&rows).Error; err != nil {
		return resp, err
	}

	resp.TotalPesananCount = int64(len(rows))
	for _, r := range rows {
		resp.TotalPesananAmount += r.TotalAmount
		resp.Items = append(resp.Items, RecapOrderItemDTO{
			PoNo:         r.PoNo,
			CustomerName: r.CustomerName,
			Date:         r.OrderDate.Local().Format("Jan 02, 2006"),
			TotalAmount:  r.TotalAmount,
			OrderStatus:  mapOrderStatus(r.OrderStatus),
		})
	}
	resp.TotalPesananAmount = roundTwo(resp.TotalPesananAmount)
	if resp.Items == nil {
		resp.Items = []RecapOrderItemDTO{}
	}
	return resp, nil
}

// ─────────────────────────────────────────────
// 4. Detail Unpaid
// ─────────────────────────────────────────────

func GetDetailUnpaid(f RecapFilter) (DetailUnpaidDTO, error) {
	var resp DetailUnpaidDTO

	start, end, err := parseRecapDates(f)
	if err != nil {
		return resp, err
	}

	rows, err := queryInvoices(start, end, f.Search, models.PaymentStatusUnpaid, models.PaymentStatusPartial)
	if err != nil {
		return resp, err
	}

	resp.TotalCount = int64(len(rows))
	for _, r := range rows {
		resp.TotalUnpaid += r.RemainingBalance
		resp.Items = append(resp.Items, RecapInvoiceItemDTO{
			PoNo:          r.PoNo,
			CustomerName:  r.CustomerName,
			Date:          r.OrderDate.Local().Format("Jan 02, 2006"),
			TotalAmount:   r.RemainingBalance,
			PaymentStatus: mapPaymentStatus(r.PaymentStatus),
		})
	}
	resp.TotalUnpaid = roundTwo(resp.TotalUnpaid)
	if resp.Items == nil {
		resp.Items = []RecapInvoiceItemDTO{}
	}
	return resp, nil
}

// ─────────────────────────────────────────────
// 5. Detail Paid
// ─────────────────────────────────────────────

func GetDetailPaid(f RecapFilter) (DetailPaidDTO, error) {
	var resp DetailPaidDTO

	start, end, err := parseRecapDates(f)
	if err != nil {
		return resp, err
	}

	rows, err := queryInvoices(start, end, f.Search, models.PaymentStatusPaid)
	if err != nil {
		return resp, err
	}

	resp.TotalCount = int64(len(rows))
	for _, r := range rows {
		resp.TotalPaid += r.TotalAmount
		resp.Items = append(resp.Items, RecapInvoiceItemDTO{
			PoNo:          r.PoNo,
			CustomerName:  r.CustomerName,
			Date:          r.OrderDate.Local().Format("Jan 02, 2006"),
			TotalAmount:   r.TotalAmount,
			PaymentStatus: "Paid",
		})
	}
	resp.TotalPaid = roundTwo(resp.TotalPaid)
	if resp.Items == nil {
		resp.Items = []RecapInvoiceItemDTO{}
	}
	return resp, nil
}
