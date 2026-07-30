package tests

import (
	"testing"
	"time"

	"teknik/config"
	"teknik/models"
	"teknik/services"
	"teknik/utils"

	"github.com/google/uuid"
)

// TestSendPaymentDueReminder menguji pengiriman email pengingat jatuh tempo pembayaran.
// Membuat pesanan yang sudah berumur 4 bulan (melebihi batas termin 3 bulan) yang belum lunas,
// lalu memicu proses pemindaian dan verifikasi email terkirim.
func TestSendPaymentDueReminder(t *testing.T) {
	helperCleanDB()
	_ = helperCreateTestUser()

	// Set tanggal pesanan 4 bulan yang lalu (sudah jatuh tempo > 3 bulan)
	fourMonthsAgo := time.Now().AddDate(0, -4, 0)
	order := models.Order{
		ID:               uuid.New(),
		TransactionNo:    "TRX-DUE-99",
		PoNo:             "PO-DUE-99",
		OrderDate:        utils.JSONDate(fourMonthsAgo),
		OrderStatus:      models.OrderStatusPending,
		RecipientName:    "PT Pelanggan Jatuh Tempo",
		RecipientEmail:   "apanyaclay1@gmail.com",
	}
	config.DB.Create(&order)

	// Buat item pesanan agar nominal tagihan > 0
	orderItem := models.OrderItem{
		ID:           uuid.New(),
		OrderID:      order.ID,
		ProductName:  "Produk Uji Jatuh Tempo",
		OrderQty:     5,
		RemainingQty: 5,
		UnitPrice:    25000,
		Subtotal:     125000,
	}
	config.DB.Create(&orderItem)

	t.Log("Memulai pemindaian transaksi jatuh tempo...")
	
	// Panggil fungsi pemindai transaksi jatuh tempo
	sentCount, err := services.Send3MonthOverduePaymentReminders()
	if err != nil {
		t.Fatalf("Gagal menjalankan pemindai jatuh tempo: %v", err)
	}

	if sentCount != 1 {
		t.Errorf("Ekspektasi email terkirim = 1, tetapi mendapatkan: %d", sentCount)
	}

	t.Logf("Sukses! %d email pengingat jatuh tempo dikirim ke apanyaclay1@gmail.com", sentCount)
}
