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
	config.DB.Model(&models.Order{}).Count(&resp.TotalPesananCount)

	// 2. Pesanan Selesai
	config.DB.Model(&models.Order{}).Where("order_status = ?", models.OrderStatusCompleted).Count(&resp.PesananSelesaiCount)

	// 3. Pesanan Di Proses (Pending + Partial)
	config.DB.Model(&models.Order{}).Where("order_status IN ?", []models.OrderStatus{models.OrderStatusPending, models.OrderStatusPartial}).Count(&resp.PesananDiprosesCount)

	// 4. Pengiriman Berlangsung / Dikirim
	config.DB.Model(&models.Shipment{}).Where("shipping_status = ?", models.ShippingStatusDikirim).Count(&resp.DikirimCount)
	resp.PengirimanBerlangsungCount = resp.DikirimCount

	// 5. Pengiriman Selesai
	config.DB.Model(&models.Shipment{}).Where("shipping_status = ?", models.ShippingStatusDiterima).Count(&resp.PengirimanSelesaiCount)

	// 6. Belum Bayar (Amount & Count)
	var unpaidInvoices []models.Invoice
	config.DB.Where("payment_status IN ?", []models.PaymentStatus{models.PaymentStatusUnpaid, models.PaymentStatusPartial}).Find(&unpaidInvoices)

	resp.BelumBayarCount = int64(len(unpaidInvoices))
	for _, inv := range unpaidInvoices {
		resp.BelumBayarAmount += inv.RemainingBalance
	}

	// 7. Aktivitas Terakhir (Mengambil 5 Shipment terbaru)
	type shipmentActivity struct {
		ShippingStatus string
		UpdatedAt      time.Time
		PoNo           string
	}
	var recents []shipmentActivity
	config.DB.Table("shipments").
		Select("shipments.shipping_status, shipments.updated_at, orders.po_no").
		Joins("JOIN orders ON orders.id = shipments.order_id").
		Order("shipments.updated_at DESC").
		Limit(5).
		Scan(&recents)

	for _, s := range recents {
		title := "Pengiriman Terkonfirmasi"
		desc := fmt.Sprintf("Pesanan %s dikirim", s.PoNo)

		if s.ShippingStatus == string(models.ShippingStatusDiterima) {
			title = "Pengiriman Selesai"
			desc = fmt.Sprintf("Pesanan %s diterima", s.PoNo)
		}

		resp.AktivitasTerakhir = append(resp.AktivitasTerakhir, DashboardActivityDTO{
			Title:       title,
			Description: desc,
			Date:        s.UpdatedAt.Format("Jan 02, 2006"),
		})
	}

	if resp.AktivitasTerakhir == nil {
		resp.AktivitasTerakhir = []DashboardActivityDTO{}
	}

	return resp, nil
}
