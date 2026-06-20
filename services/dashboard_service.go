package services

import (
	"fmt"
	"time"

	"teknik/config"
	"teknik/models"
)

// ─────────────────────────────────────────────
// Dashboard DTOs
// ─────────────────────────────────────────────

type DashboardActivityDTO struct {
	Title       string `json:"title" example:"Pengiriman Terkonfirmasi"`
	Description string `json:"description" example:"Pesanan P2397DJJ22 dikirim"`
	Date        string `json:"date" example:"Oct 23, 2026"`
}

type DashboardResponseDTO struct {
	TotalPesananCount          int64                  `json:"total_pesanan_count" example:"1284"`
	DikirimCount               int64                  `json:"dikirim_count" example:"42"`
	PesananSelesaiCount        int64                  `json:"pesanan_selesai_count" example:"1156"`
	BelumBayarAmount           float64                `json:"belum_bayar_amount" example:"12300000"`
	BelumBayarCount            int64                  `json:"belum_bayar_count" example:"8"`
	PengirimanBerlangsungCount int64                  `json:"pengiriman_berlangsung_count" example:"142"`
	PengirimanSelesaiCount     int64                  `json:"pengiriman_selesai_count" example:"1204"`
	PesananDiprosesCount       int64                  `json:"pesanan_diproses_count" example:"28"`
	AktivitasTerakhir          []DashboardActivityDTO `json:"aktivitas_terakhir"`
}

type dbActivity struct {
	ActivityType  string    `gorm:"column:activity_type"`
	Timestamp     time.Time `gorm:"column:timestamp"`
	PoNo          string    `gorm:"column:po_no"`
	TransactionNo string    `gorm:"column:transaction_no"`
	RecipientName string    `gorm:"column:recipient_name"`
	Amount        float64   `gorm:"column:amount"`
	Status        string    `gorm:"column:status"`
}

func formatRupiah(amount float64) string {
	val := int64(amount)
	s := fmt.Sprintf("%d", val)
	if len(s) <= 3 {
		return "Rp " + s
	}
	
	var result []string
	for len(s) > 3 {
		result = append([]string{s[len(s)-3:]}, result...)
		s = s[:len(s)-3]
	}
	if len(s) > 0 {
		result = append([]string{s}, result...)
	}
	
	var joined string
	for i, part := range result {
		if i == 0 {
			joined = part
		} else {
			joined += "." + part
		}
	}
	return "Rp " + joined
}

// ─────────────────────────────────────────────
// Get Dashboard Metrics
// ─────────────────────────────────────────────

func GetDashboardMetrics() (DashboardResponseDTO, error) {
	var resp DashboardResponseDTO

	// 1. Total Pesanan
	if err := config.DB.Model(&models.Order{}).Count(&resp.TotalPesananCount).Error; err != nil {
		return resp, err
	}

	// 2. Pesanan Selesai
	if err := config.DB.Model(&models.Order{}).Where("order_status = ?", models.OrderStatusCompleted).Count(&resp.PesananSelesaiCount).Error; err != nil {
		return resp, err
	}

	// 3. Pesanan Di Proses (Pending + Partial + Shipped)
	if err := config.DB.Model(&models.Order{}).Where("order_status IN ?", []models.OrderStatus{models.OrderStatusPending, models.OrderStatusPartial, models.OrderStatusShipped}).Count(&resp.PesananDiprosesCount).Error; err != nil {
		return resp, err
	}

	// 4. Pengiriman Berlangsung / Dikirim
	if err := config.DB.Model(&models.Shipment{}).Where("shipping_status = ?", models.ShippingStatusDikirim).Count(&resp.DikirimCount).Error; err != nil {
		return resp, err
	}
	resp.PengirimanBerlangsungCount = resp.DikirimCount

	// 5. Pengiriman Selesai
	if err := config.DB.Model(&models.Shipment{}).Where("shipping_status = ?", models.ShippingStatusDiterima).Count(&resp.PengirimanSelesaiCount).Error; err != nil {
		return resp, err
	}

	// 6. Belum Bayar (Amount & Count) - Dioptimalkan dengan agregasi tingkat database untuk performa maksimal
	var unpaidStats struct {
		Count  int64
		Amount float64
	}
	if err := config.DB.Model(&models.Invoice{}).
		Select("COUNT(*) AS count, COALESCE(SUM(remaining_balance), 0) AS amount").
		Where("payment_status IN ?", []models.PaymentStatus{models.PaymentStatusUnpaid, models.PaymentStatusPartial}).
		Scan(&unpaidStats).Error; err != nil {
		return resp, err
	}
	resp.BelumBayarCount = unpaidStats.Count
	resp.BelumBayarAmount = unpaidStats.Amount

	// 7. Aktivitas Terakhir (Mengambil 10 aktivitas gabungan terbaru menggunakan SQL UNION)
	var rows []dbActivity
	unionQuery := `
		SELECT 'shipment' AS activity_type, shipments.created_at AS timestamp, orders.po_no, orders.transaction_no, orders.recipient_name, 0.0 AS amount, 'dikirim' AS status
		FROM shipments
		JOIN orders ON orders.id = shipments.order_id AND orders.deleted_at IS NULL
		WHERE shipments.deleted_at IS NULL

		UNION ALL

		SELECT 'shipment' AS activity_type, shipments.updated_at AS timestamp, orders.po_no, orders.transaction_no, orders.recipient_name, 0.0 AS amount, 'diterima' AS status
		FROM shipments
		JOIN orders ON orders.id = shipments.order_id AND orders.deleted_at IS NULL
		WHERE shipments.deleted_at IS NULL AND shipments.shipping_status = 'diterima'

		UNION ALL

		SELECT 'order' AS activity_type, orders.created_at AS timestamp, orders.po_no, orders.transaction_no, orders.recipient_name, 0.0 AS amount, 'pending' AS status
		FROM orders
		WHERE orders.deleted_at IS NULL

		UNION ALL

		SELECT 'order' AS activity_type, orders.updated_at AS timestamp, orders.po_no, orders.transaction_no, orders.recipient_name, 0.0 AS amount, 'completed' AS status
		FROM orders
		WHERE orders.deleted_at IS NULL AND orders.order_status = 'completed'

		UNION ALL

		SELECT 'payment' AS activity_type, payments.created_at AS timestamp, orders.po_no, orders.transaction_no, orders.recipient_name, payment_details.allocated_amount AS amount, 'diterima' AS status
		FROM payments
		JOIN payment_details ON payment_details.payment_id = payments.id AND payment_details.deleted_at IS NULL
		JOIN invoices ON invoices.id = payment_details.invoice_id AND invoices.deleted_at IS NULL
		JOIN shipments ON shipments.id = invoices.shipment_id AND shipments.deleted_at IS NULL
		JOIN orders ON orders.id = shipments.order_id AND orders.deleted_at IS NULL
		WHERE payments.deleted_at IS NULL

		UNION ALL

		SELECT 'payment' AS activity_type, payments.updated_at AS timestamp, orders.po_no, orders.transaction_no, orders.recipient_name, payment_details.allocated_amount AS amount, 'diperbarui' AS status
		FROM payments
		JOIN payment_details ON payment_details.payment_id = payments.id AND payment_details.deleted_at IS NULL
		JOIN invoices ON invoices.id = payment_details.invoice_id AND invoices.deleted_at IS NULL
		JOIN shipments ON shipments.id = invoices.shipment_id AND shipments.deleted_at IS NULL
		JOIN orders ON orders.id = shipments.order_id AND orders.deleted_at IS NULL
		WHERE payments.deleted_at IS NULL AND payments.updated_at > payments.created_at
	`
	paginatedQuery := fmt.Sprintf("SELECT * FROM (%s) AS combined_activities ORDER BY timestamp DESC LIMIT 10", unionQuery)
	if err := config.DB.Raw(paginatedQuery).Scan(&rows).Error; err != nil {
		return resp, err
	}

	resp.AktivitasTerakhir = make([]DashboardActivityDTO, 0, len(rows))
	for _, r := range rows {
		var title string
		var desc string
		
		var identifier string
		if r.PoNo != "" {
			identifier = fmt.Sprintf("%s dengan No Transaksi %s (PO: %s)", r.RecipientName, r.TransactionNo, r.PoNo)
		} else {
			identifier = fmt.Sprintf("%s dengan No Transaksi %s", r.RecipientName, r.TransactionNo)
		}

		switch r.ActivityType {
		case "shipment":
			title = "Pengiriman Terkonfirmasi"
			desc = fmt.Sprintf("Pesanan %s dikirim", identifier)
			if r.Status == string(models.ShippingStatusDiterima) {
				title = "Pengiriman Selesai"
				desc = fmt.Sprintf("Pesanan %s diterima", identifier)
			}
		case "order":
			title = "Pesanan Baru"
			desc = fmt.Sprintf("Pesanan %s telah dibuat", identifier)
			if r.Status == "completed" {
				title = "Pesanan Selesai"
				desc = fmt.Sprintf("Pesanan %s telah selesai diproses", identifier)
			}
		case "payment":
			title = "Pembayaran Diterima"
			formattedAmount := formatRupiah(r.Amount)
			desc = fmt.Sprintf("Pembayaran sebesar %s untuk pesanan %s berhasil diproses", formattedAmount, identifier)
			if r.Status == "diperbarui" {
				title = "Pembayaran Diperbarui"
				desc = fmt.Sprintf("Pembayaran sebesar %s untuk pesanan %s telah diperbarui", formattedAmount, identifier)
			}
		}

		resp.AktivitasTerakhir = append(resp.AktivitasTerakhir, DashboardActivityDTO{
			Title:       title,
			Description: desc,
			Date:        r.Timestamp.Local().Format("2006-01-02"),
		})
	}

	if resp.AktivitasTerakhir == nil {
		resp.AktivitasTerakhir = []DashboardActivityDTO{}
	}

	return resp, nil
}

// ─────────────────────────────────────────────
// Get All Dashboard Activities
// ─────────────────────────────────────────────

func GetAllDashboardActivities(startDate, endDate string, page, limit int) ([]DashboardActivityDTO, int64, error) {
	unionQuery := `
		SELECT 'shipment' AS activity_type, shipments.created_at AS timestamp, orders.po_no, orders.transaction_no, orders.recipient_name, 0.0 AS amount, 'dikirim' AS status
		FROM shipments
		JOIN orders ON orders.id = shipments.order_id AND orders.deleted_at IS NULL
		WHERE shipments.deleted_at IS NULL

		UNION ALL

		SELECT 'shipment' AS activity_type, shipments.updated_at AS timestamp, orders.po_no, orders.transaction_no, orders.recipient_name, 0.0 AS amount, 'diterima' AS status
		FROM shipments
		JOIN orders ON orders.id = shipments.order_id AND orders.deleted_at IS NULL
		WHERE shipments.deleted_at IS NULL AND shipments.shipping_status = 'diterima'

		UNION ALL

		SELECT 'order' AS activity_type, orders.created_at AS timestamp, orders.po_no, orders.transaction_no, orders.recipient_name, 0.0 AS amount, 'pending' AS status
		FROM orders
		WHERE orders.deleted_at IS NULL

		UNION ALL

		SELECT 'order' AS activity_type, orders.updated_at AS timestamp, orders.po_no, orders.transaction_no, orders.recipient_name, 0.0 AS amount, 'completed' AS status
		FROM orders
		WHERE orders.deleted_at IS NULL AND orders.order_status = 'completed'

		UNION ALL

		SELECT 'payment' AS activity_type, payments.created_at AS timestamp, orders.po_no, orders.transaction_no, orders.recipient_name, payment_details.allocated_amount AS amount, 'diterima' AS status
		FROM payments
		JOIN payment_details ON payment_details.payment_id = payments.id AND payment_details.deleted_at IS NULL
		JOIN invoices ON invoices.id = payment_details.invoice_id AND invoices.deleted_at IS NULL
		JOIN shipments ON shipments.id = invoices.shipment_id AND shipments.deleted_at IS NULL
		JOIN orders ON orders.id = shipments.order_id AND orders.deleted_at IS NULL
		WHERE payments.deleted_at IS NULL

		UNION ALL

		SELECT 'payment' AS activity_type, payments.updated_at AS timestamp, orders.po_no, orders.transaction_no, orders.recipient_name, payment_details.allocated_amount AS amount, 'diperbarui' AS status
		FROM payments
		JOIN payment_details ON payment_details.payment_id = payments.id AND payment_details.deleted_at IS NULL
		JOIN invoices ON invoices.id = payment_details.invoice_id AND invoices.deleted_at IS NULL
		JOIN shipments ON shipments.id = invoices.shipment_id AND shipments.deleted_at IS NULL
		JOIN orders ON orders.id = shipments.order_id AND orders.deleted_at IS NULL
		WHERE payments.deleted_at IS NULL AND payments.updated_at > payments.created_at
	`

	var startLimit time.Time
	var endLimit time.Time
	var hasStart, hasEnd bool
	layout := "2006-01-02"

	if startDate != "" {
		t, err := time.Parse(layout, startDate)
		if err != nil {
			return nil, 0, fmt.Errorf("format start_date tidak valid, gunakan YYYY-MM-DD")
		}
		startLimit = t
		hasStart = true
	}
	if endDate != "" {
		t, err := time.Parse(layout, endDate)
		if err != nil {
			return nil, 0, fmt.Errorf("format end_date tidak valid, gunakan YYYY-MM-DD")
		}
		endLimit = t.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
		hasEnd = true
	}

	var args []interface{}
	whereClause := ""

	if hasStart {
		whereClause += " WHERE timestamp >= ?"
		args = append(args, startLimit)
	}
	if hasEnd {
		if whereClause == "" {
			whereClause += " WHERE timestamp <= ?"
		} else {
			whereClause += " AND timestamp <= ?"
		}
		args = append(args, endLimit)
	}

	// Hitung total data untuk keperluan paginasi
	var totalRows int64
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM (%s) AS combined_activities%s", unionQuery, whereClause)
	if err := config.DB.Raw(countQuery, args...).Scan(&totalRows).Error; err != nil {
		return nil, 0, err
	}

	// Ambil data terpaginasi
	offset := (page - 1) * limit
	paginatedQuery := fmt.Sprintf("SELECT * FROM (%s) AS combined_activities%s ORDER BY timestamp DESC LIMIT ? OFFSET ?", unionQuery, whereClause)
	
	paginatedArgs := append(args, limit, offset)
	var rows []dbActivity
	if err := config.DB.Raw(paginatedQuery, paginatedArgs...).Scan(&rows).Error; err != nil {
		return nil, 0, err
	}

	activities := make([]DashboardActivityDTO, 0, len(rows))
	for _, r := range rows {
		var title string
		var desc string
		
		var identifier string
		if r.PoNo != "" {
			identifier = fmt.Sprintf("%s dengan No Transaksi %s (PO: %s)", r.RecipientName, r.TransactionNo, r.PoNo)
		} else {
			identifier = fmt.Sprintf("%s dengan No Transaksi %s", r.RecipientName, r.TransactionNo)
		}

		switch r.ActivityType {
		case "shipment":
			title = "Pengiriman Terkonfirmasi"
			desc = fmt.Sprintf("Pesanan %s dikirim", identifier)
			if r.Status == string(models.ShippingStatusDiterima) {
				title = "Pengiriman Selesai"
				desc = fmt.Sprintf("Pesanan %s diterima", identifier)
			}
		case "order":
			title = "Pesanan Baru"
			desc = fmt.Sprintf("Pesanan %s telah dibuat", identifier)
			if r.Status == "completed" {
				title = "Pesanan Selesai"
				desc = fmt.Sprintf("Pesanan %s telah selesai diproses", identifier)
			}
		case "payment":
			title = "Pembayaran Diterima"
			formattedAmount := formatRupiah(r.Amount)
			desc = fmt.Sprintf("Pembayaran sebesar %s untuk pesanan %s berhasil diproses", formattedAmount, identifier)
			if r.Status == "diperbarui" {
				title = "Pembayaran Diperbarui"
				desc = fmt.Sprintf("Pembayaran sebesar %s untuk pesanan %s telah diperbarui", formattedAmount, identifier)
			}
		}

		activities = append(activities, DashboardActivityDTO{
			Title:       title,
			Description: desc,
			Date:        r.Timestamp.Local().Format("2006-01-02"),
		})
	}

	if activities == nil {
		activities = []DashboardActivityDTO{}
	}

	return activities, totalRows, nil
}

