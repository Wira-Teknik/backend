package tests

import (
	"fmt"
	"os"
	"testing"
	"time"

	"teknik/config"
	"teknik/models"
	"teknik/services"
	"teknik/utils"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
)

// TestMain mengatur inisialisasi database khusus untuk testing.
func TestMain(m *testing.M) {
	// Memuat file env
	_ = godotenv.Load("../.env")

	// Selalu paksa menggunakan database testing agar database development tidak kotor
	originalDBName := os.Getenv("DB_NAME")
	if originalDBName == "" {
		originalDBName = "wira_teknik"
	}
	os.Setenv("DB_NAME", originalDBName+"_test")

	// Hubungkan database
	config.ConnectDatabase()

	// Jalankan migrasi schema
	if err := config.DB.AutoMigrate(
		&models.User{},
		&models.Customer{},
		&models.Order{},
		&models.OrderItem{},
		&models.Payment{},
		&models.PaymentDetail{},
		&models.Shipment{},
		&models.ShipmentItem{},
		&models.Invoice{},
		&models.Attachment{},
		&models.AuditLog{},
	); err != nil {
		fmt.Printf("Migrasi testing gagal: %v\n", err)
		os.Exit(1)
	}

	// Jalankan suite pengujian
	code := m.Run()

	// Beri jeda 8 detik agar goroutine email di background selesai terkirim
	fmt.Println("\nMenunggu seluruh pengiriman email latar belakang selesai...")
	time.Sleep(8 * time.Second)

	// Selesai pengujian
	os.Exit(code)
}

// helperCleanDB membersihkan tabel sebelum pengujian dijalankan.
func helperCleanDB() {
	config.DB.Exec("TRUNCATE TABLE audit_logs CASCADE")
	config.DB.Exec("TRUNCATE TABLE attachments CASCADE")
	config.DB.Exec("TRUNCATE TABLE payment_details CASCADE")
	config.DB.Exec("TRUNCATE TABLE payments CASCADE")
	config.DB.Exec("TRUNCATE TABLE invoices CASCADE")
	config.DB.Exec("TRUNCATE TABLE shipment_items CASCADE")
	config.DB.Exec("TRUNCATE TABLE shipments CASCADE")
	config.DB.Exec("TRUNCATE TABLE order_items CASCADE")
	config.DB.Exec("TRUNCATE TABLE orders CASCADE")
	config.DB.Exec("TRUNCATE TABLE customers CASCADE")
	config.DB.Exec("TRUNCATE TABLE users CASCADE")
}

// helperCreateTestUser membuat data user testing di database agar relasi foreign key pada audit_logs valid.
func helperCreateTestUser() uuid.UUID {
	userID := uuid.New()
	user := models.User{
		ID:       userID,
		Name:     "testuser",
		Email:    "apanyaclay1@gmail.com",
		Password: "hashedpassword123",
		Role:     models.RoleAdmin,
	}
	config.DB.Create(&user)
	return userID
}

// TestShipmentValidationDate menguji aturan bisnis bahwa tanggal kirim tidak boleh sebelum tanggal PO.
func TestShipmentValidationDate(t *testing.T) {
	helperCleanDB()
	dummyUserID := helperCreateTestUser()

	// Setup data dasar
	cust := models.Customer{
		ID:           uuid.New(),
		CustomerName: "PT Uji Coba Tanggal",
	}
	config.DB.Create(&cust)

	orderDate := time.Date(2026, 6, 10, 0, 0, 0, 0, time.Local)
	order := models.Order{
		ID:               uuid.New(),
		TransactionNo:    "TRX-TGL-01",
		OrderDate:        utils.JSONDate(orderDate),
		OrderStatus:      models.OrderStatusPending,
		RecipientName:    "PT Uji Coba Tanggal",
		RecipientEmail:   "apanyaclay1@gmail.com",
	}
	config.DB.Create(&order)

	orderItem := models.OrderItem{
		ID:           uuid.New(),
		OrderID:      order.ID,
		ProductName:  "Produk Tanggal",
		OrderQty:     10,
		RemainingQty: 10,
		UnitPrice:    10000,
	}
	config.DB.Create(&orderItem)

	// Uji Tanggal Kirim Sebelum Tanggal Order (Kasus Gagal)
	invalidInput := services.CreateShipmentInput{
		OrderID:      order.ID.String(),
		ShippingDate: "2026-06-09", // Sebelum 2026-06-10
		Items: []services.ShipmentItemInput{
			{OrderItemID: orderItem.ID.String(), ShippingQty: 5},
		},
	}

	_, err := services.CreateShipment(invalidInput, dummyUserID)
	if err == nil {
		t.Errorf("Ekspektasi error karena tanggal kirim sebelum tanggal order, tetapi tidak ada error")
	} else if err != services.ErrShipmentDateBeforeOrderDate {
		t.Errorf("Ekspektasi error %v, tetapi mendapatkan: %v", services.ErrShipmentDateBeforeOrderDate, err)
	}

	// Uji Tanggal Kirim Sama dengan Tanggal Order (Kasus Sukses)
	validInput := services.CreateShipmentInput{
		OrderID:      order.ID.String(),
		ShippingDate: "2026-06-10",
		Items: []services.ShipmentItemInput{
			{OrderItemID: orderItem.ID.String(), ShippingQty: 5},
		},
	}
	_, err = services.CreateShipment(validInput, dummyUserID)
	if err != nil {
		t.Errorf("Ekspektasi pengiriman sukses, tetapi mendapatkan error: %v", err)
	}
}

// TestShipmentQtyLimit menguji aturan bahwa kuantitas kirim tidak boleh melebihi sisa pesanan.
func TestShipmentQtyLimit(t *testing.T) {
	helperCleanDB()
	dummyUserID := helperCreateTestUser()

	// Setup data dasar
	cust := models.Customer{
		ID:           uuid.New(),
		CustomerName: "PT Uji Coba Stok",
	}
	config.DB.Create(&cust)

	orderDate := time.Date(2026, 6, 10, 0, 0, 0, 0, time.Local)
	order := models.Order{
		ID:               uuid.New(),
		TransactionNo:    "TRX-STK-01",
		OrderDate:        utils.JSONDate(orderDate),
		OrderStatus:      models.OrderStatusPending,
		RecipientName:    "PT Uji Coba Stok",
		RecipientEmail:   "apanyaclay1@gmail.com",
	}
	config.DB.Create(&order)

	orderItem := models.OrderItem{
		ID:           uuid.New(),
		OrderID:      order.ID,
		ProductName:  "Produk Stok",
		OrderQty:     10,
		RemainingQty: 10,
		UnitPrice:    10000,
	}
	config.DB.Create(&orderItem)

	// Uji kuantitas kirim berlebih
	invalidInput := services.CreateShipmentInput{
		OrderID:      order.ID.String(),
		ShippingDate: "2026-06-11",
		Items: []services.ShipmentItemInput{
			{OrderItemID: orderItem.ID.String(), ShippingQty: 12}, // Melebihi remaining_qty (10)
		},
	}

	_, err := services.CreateShipment(invalidInput, dummyUserID)
	if err == nil {
		t.Errorf("Ekspektasi error karena kuantitas melebihi sisa pesanan, tetapi tidak ada error")
	}
}

// TestOrderStatusTransitions menguji perpindahan status order secara sekuensial.
func TestOrderStatusTransitions(t *testing.T) {
	helperCleanDB()
	dummyUserID := helperCreateTestUser()

	cust := models.Customer{
		ID:           uuid.New(),
		CustomerName: "PT Uji Coba Status",
	}
	config.DB.Create(&cust)

	orderDate := time.Date(2026, 6, 10, 0, 0, 0, 0, time.Local)
	order := models.Order{
		ID:               uuid.New(),
		TransactionNo:    "TRX-STS-01",
		OrderDate:        utils.JSONDate(orderDate),
		OrderStatus:      models.OrderStatusPending,
		RecipientName:    "PT Uji Coba Status",
		RecipientEmail:   "apanyaclay1@gmail.com",
	}
	config.DB.Create(&order)

	orderItem := models.OrderItem{
		ID:           uuid.New(),
		OrderID:      order.ID,
		ProductName:  "Produk Status",
		OrderQty:     10,
		RemainingQty: 10,
		UnitPrice:    10000,
	}
	config.DB.Create(&orderItem)

	// 1. Pengiriman Parsial pertama (Kirim 4 pcs) -> Status harus 'partial'
	shipInput1 := services.CreateShipmentInput{
		OrderID:      order.ID.String(),
		ShippingDate: "2026-06-11",
		Items: []services.ShipmentItemInput{
			{OrderItemID: orderItem.ID.String(), ShippingQty: 4},
		},
	}
	ship1, err := services.CreateShipment(shipInput1, dummyUserID)
	if err != nil {
		t.Fatalf("Gagal membuat pengiriman 1: %v", err)
	}

	var updatedOrder models.Order
	config.DB.First(&updatedOrder, "id = ?", order.ID)
	if updatedOrder.OrderStatus != models.OrderStatusPartial {
		t.Errorf("Ekspektasi status 'partial', tetapi mendapatkan '%s'", updatedOrder.OrderStatus)
	}

	// 2. Pengiriman Parsial kedua (Kirim sisa 6 pcs) -> Status harus 'shipped'
	shipInput2 := services.CreateShipmentInput{
		OrderID:      order.ID.String(),
		ShippingDate: "2026-06-12",
		Items: []services.ShipmentItemInput{
			{OrderItemID: orderItem.ID.String(), ShippingQty: 6},
		},
	}
	ship2, err := services.CreateShipment(shipInput2, dummyUserID)
	if err != nil {
		t.Fatalf("Gagal membuat pengiriman 2: %v", err)
	}

	config.DB.First(&updatedOrder, "id = ?", order.ID)
	if updatedOrder.OrderStatus != models.OrderStatusShipped {
		t.Errorf("Ekspektasi status 'shipped', tetapi mendapatkan '%s'", updatedOrder.OrderStatus)
	}

	// 3. Konfirmasi pengiriman 1 diterima -> Status harus tetap 'shipped' (karena pengiriman 2 belum diterima)
	_, err = services.ConfirmShipmentReceived(ship1.ID.String(), dummyUserID)
	if err != nil {
		t.Fatalf("Gagal konfirmasi pengiriman 1 diterima: %v", err)
	}

	config.DB.First(&updatedOrder, "id = ?", order.ID)
	if updatedOrder.OrderStatus != models.OrderStatusShipped {
		t.Errorf("Ekspektasi status tetap 'shipped', tetapi mendapatkan '%s'", updatedOrder.OrderStatus)
	}

	// 4. Konfirmasi pengiriman 2 diterima -> Status harus 'completed'
	_, err = services.ConfirmShipmentReceived(ship2.ID.String(), dummyUserID)
	if err != nil {
		t.Fatalf("Gagal konfirmasi pengiriman 2 diterima: %v", err)
	}

	config.DB.First(&updatedOrder, "id = ?", order.ID)
	if updatedOrder.OrderStatus != models.OrderStatusCompleted {
		t.Errorf("Ekspektasi status 'completed', tetapi mendapatkan '%s'", updatedOrder.OrderStatus)
	}
}

// TestPaymentAllocationFIFO menguji alokasi FIFO nominal pembayaran ke beberapa invoice.
func TestPaymentAllocationFIFO(t *testing.T) {
	helperCleanDB()
	dummyUserID := helperCreateTestUser()

	cust := models.Customer{
		ID:           uuid.New(),
		CustomerName: "PT Uji Coba FIFO",
	}
	config.DB.Create(&cust)

	orderDate := time.Date(2026, 6, 10, 0, 0, 0, 0, time.Local)
	order := models.Order{
		ID:               uuid.New(),
		TransactionNo:    "TRX-FIFO-01",
		OrderDate:        utils.JSONDate(orderDate),
		OrderStatus:      models.OrderStatusPending,
		RecipientName:    "PT Uji Coba FIFO",
		RecipientEmail:   "apanyaclay1@gmail.com",
	}
	config.DB.Create(&order)

	// Buat item unit price = 1000 agar gampang dihitung
	orderItem := models.OrderItem{
		ID:           uuid.New(),
		OrderID:      order.ID,
		ProductName:  "Produk FIFO",
		OrderQty:     10,
		RemainingQty: 10,
		UnitPrice:    1000,
	}
	config.DB.Create(&orderItem)

	// Kirim 2 kali secara parsial untuk memicu 2 invoice
	// PPN rate = 11% (maka Total Invoice = Qty * UnitPrice * 1.11)
	// Kirim 3 pcs (Invoice 1 = 3 * 1000 * 1.11 = 3330)
	shipInput1 := services.CreateShipmentInput{
		OrderID:      order.ID.String(),
		ShippingDate: "2026-06-11",
		Items: []services.ShipmentItemInput{
			{OrderItemID: orderItem.ID.String(), ShippingQty: 3},
		},
	}
	ship1, _ := services.CreateShipment(shipInput1, dummyUserID)

	// Kirim 4 pcs (Invoice 2 = 4 * 1000 * 1.11 = 4440)
	shipInput2 := services.CreateShipmentInput{
		OrderID:      order.ID.String(),
		ShippingDate: "2026-06-12",
		Items: []services.ShipmentItemInput{
			{OrderItemID: orderItem.ID.String(), ShippingQty: 4},
		},
	}
	ship2, _ := services.CreateShipment(shipInput2, dummyUserID)

	// Ambil invoice
	var inv1, inv2 models.Invoice
	config.DB.First(&inv1, "shipment_id = ?", ship1.ID)
	config.DB.First(&inv2, "shipment_id = ?", ship2.ID)

	// Bayar 5000 (Melunasi Invoice 1 [3330], sisa 1670 teralokasi ke Invoice 2 [4440])
	payInput := services.CreatePaymentInput{
		PaymentDate:  "2026-06-15",
		PaymentTotal: 5000,
		OrderIDs:     []string{order.ID.String()},
	}

	_, err := services.CreatePayment(payInput, dummyUserID)
	if err != nil {
		t.Fatalf("Gagal membuat pembayaran: %v", err)
	}

	// Verifikasi Invoice 1
	config.DB.First(&inv1, "id = ?", inv1.ID)
	if inv1.RemainingBalance != 0 || inv1.PaymentStatus != models.PaymentStatusPaid {
		t.Errorf("Ekspektasi Invoice 1 lunas (0 unpaid), tetapi sisa saldo: %.2f status: %s", inv1.RemainingBalance, inv1.PaymentStatus)
	}

	// Verifikasi Invoice 2 (saldo awal 4440 - sisa bayar 1670 = 2770)
	config.DB.First(&inv2, "id = ?", inv2.ID)
	expectedBal := 4440.0 - 1670.0
	if inv2.RemainingBalance != expectedBal || inv2.PaymentStatus != models.PaymentStatusPartial {
		t.Errorf("Ekspektasi Invoice 2 sisa saldo %.2f status 'partial', tetapi sisa saldo: %.2f status: %s", expectedBal, inv2.RemainingBalance, inv2.PaymentStatus)
	}
}

// TestPaymentEditRollback menguji rollback saat nominal edit pembayaran berubah.
func TestPaymentEditRollback(t *testing.T) {
	helperCleanDB()
	dummyUserID := helperCreateTestUser()

	cust := models.Customer{
		ID:           uuid.New(),
		CustomerName: "PT Uji Coba Rollback",
	}
	config.DB.Create(&cust)

	orderDate := time.Date(2026, 6, 10, 0, 0, 0, 0, time.Local)
	order := models.Order{
		ID:               uuid.New(),
		TransactionNo:    "TRX-RB-01",
		OrderDate:        utils.JSONDate(orderDate),
		OrderStatus:      models.OrderStatusPending,
		RecipientName:    "PT Uji Coba Rollback",
		RecipientEmail:   "apanyaclay1@gmail.com",
	}
	config.DB.Create(&order)

	orderItem := models.OrderItem{
		ID:           uuid.New(),
		OrderID:      order.ID,
		ProductName:  "Produk Rollback",
		OrderQty:     10,
		RemainingQty: 10,
		UnitPrice:    1000,
	}
	config.DB.Create(&orderItem)

	// Kirim 3 pcs (Invoice 1 = 3330)
	shipInput1 := services.CreateShipmentInput{
		OrderID:      order.ID.String(),
		ShippingDate: "2026-06-11",
		Items: []services.ShipmentItemInput{
			{OrderItemID: orderItem.ID.String(), ShippingQty: 3},
		},
	}
	ship1, _ := services.CreateShipment(shipInput1, dummyUserID)

	// Kirim 4 pcs (Invoice 2 = 4440)
	shipInput2 := services.CreateShipmentInput{
		OrderID:      order.ID.String(),
		ShippingDate: "2026-06-12",
		Items: []services.ShipmentItemInput{
			{OrderItemID: orderItem.ID.String(), ShippingQty: 4},
		},
	}
	ship2, _ := services.CreateShipment(shipInput2, dummyUserID)

	var inv1, inv2 models.Invoice
	config.DB.First(&inv1, "shipment_id = ?", ship1.ID)
	config.DB.First(&inv2, "shipment_id = ?", ship2.ID)

	// Bayar awal 5000
	payInput := services.CreatePaymentInput{
		PaymentDate:  "2026-06-15",
		PaymentTotal: 5000,
		OrderIDs:     []string{order.ID.String()},
	}
	payment, _ := services.CreatePayment(payInput, dummyUserID)

	// Edit pembayaran menjadi 7770 (Melunasi Invoice 1 [3330] & Invoice 2 [4440])
	_, err := services.UpdatePaymentTotal(payment.ID.String(), 7770, []string{order.ID.String()}, dummyUserID)
	if err != nil {
		t.Fatalf("Gagal melakukan update nominal pembayaran: %v", err)
	}

	// Verifikasi Invoice 1 harus Lunas (3330 - 3330 = 0)
	config.DB.First(&inv1, "id = ?", inv1.ID)
	if inv1.RemainingBalance != 0 || inv1.PaymentStatus != models.PaymentStatusPaid {
		t.Errorf("Setelah edit, ekspektasi Invoice 1 lunas, tetapi sisa saldo: %.2f status: %s", inv1.RemainingBalance, inv1.PaymentStatus)
	}

	// Verifikasi Invoice 2 harus Lunas (4440 - 4440 = 0)
	config.DB.First(&inv2, "id = ?", inv2.ID)
	if inv2.RemainingBalance != 0 || inv2.PaymentStatus != models.PaymentStatusPaid {
		t.Errorf("Setelah edit, ekspektasi Invoice 2 lunas, tetapi sisa saldo: %.2f status: %s", inv2.RemainingBalance, inv2.PaymentStatus)
	}
}
