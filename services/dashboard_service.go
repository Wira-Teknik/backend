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

	// 7. Aktivitas Terakhir (Mengambil 10 Shipment terbaru yang tidak di-soft-delete)
	type shipmentActivity struct {
		ShippingStatus string
		UpdatedAt      time.Time
		PoNo           string
		TransactionNo  string
	}
	var recents []shipmentActivity
	if err := config.DB.Model(&models.Shipment{}).
		Select("shipments.shipping_status, shipments.updated_at, orders.po_no, orders.transaction_no").
		Joins("JOIN orders ON orders.id = shipments.order_id AND orders.deleted_at IS NULL").
		Order("shipments.updated_at DESC").
		Limit(10).
		Scan(&recents).Error; err != nil {
		return resp, err
	}

	for _, s := range recents {
		title := "Pengiriman Terkonfirmasi"
		
		// Gunakan transaction_no sebagai identitas utama, jika ada po_no tampilkan juga
		var identifier string
		if s.PoNo != "" {
			identifier = fmt.Sprintf("%s (PO: %s)", s.TransactionNo, s.PoNo)
		} else {
			identifier = s.TransactionNo
		}
		
		desc := fmt.Sprintf("Pesanan %s dikirim", identifier)

		if s.ShippingStatus == string(models.ShippingStatusDiterima) {
			title = "Pengiriman Selesai"
			desc = fmt.Sprintf("Pesanan %s diterima", identifier)
		}

		resp.AktivitasTerakhir = append(resp.AktivitasTerakhir, DashboardActivityDTO{
			Title:       title,
			Description: desc,
			Date:        s.UpdatedAt.Local().Format("2006-01-02"),
		})
	}

	if resp.AktivitasTerakhir == nil {
		resp.AktivitasTerakhir = []DashboardActivityDTO{}
	}

	return resp, nil
}
