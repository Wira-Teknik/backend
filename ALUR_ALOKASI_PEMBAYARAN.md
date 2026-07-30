# Penjelasan Algoritma Pembayaran & Alokasi Tagihan - Wira Teknik

Dokumen ini menjelaskan secara detail logika pemrograman untuk proses pembuatan dan pembaruan pembayaran pada berkas [services/payment_service.go](file:///c:/Users/apany/Documents/GitHub/golang/teknik/services/payment_service.go).

---

## 1. Pembuatan Pembayaran (`CreatePayment`)

Fungsi `CreatePayment` didefinisikan pada baris **309 hingga 456**. Alur eksekusi kodenya dibagi menjadi beberapa blok tanggung jawab berikut:

### A. Validasi Input & Parsing UUID
* **Baris 310 - 330**: 
  * Memastikan parameter `OrderIDs` tidak kosong dan nominal pembayaran `PaymentTotal` bernilai positif (> 0).
  * Melakukan parsing string ID pesanan ke tipe data `uuid.UUID` guna menghindari crash/error sintaksis query pada PostgreSQL.

### B. Query Tagihan Belum Lunas (FIFO)
* **Baris 332 - 348**:
  * Mengambil seluruh data invoice yang status pembayarannya belum lunas (`payment_status != 'paid'`) dari pesanan terpilih.
  * Menerapkan metode **FIFO (First In, First Out)** menggunakan pengurutan `invoices.created_at ASC` sehingga tagihan yang terbit lebih lama akan dilunasi terlebih dahulu.

### C. Validasi Batas Nominal Pembayaran
* **Baris 349 - 358**:
  * Menghitung total sisa tagihan dari seluruh invoice yang terkumpul.
  * Memastikan nominal pembayaran yang dimasukkan tidak melebihi sisa tagihan tersebut (`input.PaymentTotal > totalTagihan`). Jika melebihi, sistem mengembalikan error.

### D. Perhitungan Alokasi Dana (Dry Run)
* **Baris 359 - 396**:
  * Melakukan simulasi alokasi dana secara kronologis.
  * Mengurangi sisa dana pembayaran bertahap untuk melunasi tagihan demi tagihan.
  * Menyusun baris rincian data baru (`models.PaymentDetail`) dan menampungnya di memori sebelum ditulis ke database.

### E. Transaksi Database & Pembaruan Invoice
* **Baris 397 - 438**:
  * Membuka transaksi database baru (`tx := config.DB.Begin()`).
  * Menyimpan data utama pembayaran (`payments`) dan rincian alokasi (`payment_details`).
  * Memperbarui nominal sisa tagihan (`remaining_balance`) dan status pembayaran (`payment_status`) pada masing-masing invoice:
    * Status menjadi `paid` jika sisa tagihan = 0.
    * Status menjadi `partial` jika sisa tagihan > 0.
  * Jika terjadi kegagalan di salah satu proses, transaksi dibatalkan sepenuhnya (`tx.Rollback()`).

### F. Finalisasi & Notifikasi Asinkron
* **Baris 440 - 454**:
  * Melakukan commit perubahan ke database (`tx.Commit()`).
  * Memicu pengiriman email pemberitahuan pelunasan tagihan secara asinkron menggunakan goroutine (`go func() ...`).
  * Mencatat aktivitas pembuatan ke dalam sistem audit log (`CreateAuditLog`).

---

## 2. Pembaruan Pembayaran (`UpdatePaymentTotal`)

Fungsi `UpdatePaymentTotal` didefinisikan pada baris **459 hingga 600** (dan berlanjut ke baris berikutnya). Fungsi ini mengimplementasikan logika rollback keadaan sebelum mengalokasikan ulang nominal pembayaran baru secara atomik.

### A. Persiapan Transaksi & Proteksi Panic
* **Baris 460 - 486**:
  * Memvalidasi parameter masukan dan memulai transaksi database (`tx := config.DB.Begin()`).
  * Menggunakan fungsi `defer` recovery untuk mendeteksi panic runtime, sehingga jika server mengalami error mendadak, koneksi database langsung di-rollback agar tidak terjadi kebocoran data.

### B. Tahap Rollback / Pemulihan Tagihan Lama (Revert)
* **Baris 487 - 542**:
  * Mengambil rincian alokasi lama (`payment_details`) yang pernah terasosiasi dengan `payment_id` yang sedang diubah.
  * **Mengembalikan saldo tagihan**: Menambahkan kembali nominal alokasi lama ke sisa tagihan masing-masing invoice terkait:
    $$\text{invoice.remaining\_balance} = \text{invoice.remaining\_balance} + \text{detail.allocated\_amount}$$
  * **Mengatur ulang status**: Menilai kembali status pembayaran invoice tersebut (kembali ke `unpaid` jika saldo pulih sepenuhnya, atau `partial` jika masih ada sisa pembayaran dari transaksi lain).
  * Menyimpan perubahan invoice lama kembali ke database.

### C. Tahap Alokasi Ulang (Re-Allocation)
* **Baris 544 - 600**:
  * Melakukan query ulang untuk mengumpulkan tagihan-tagihan belum lunas yang baru dari pesanan yang dipilih saat ini.
  * Menghapus seluruh baris rincian alokasi lama (`payment_details`) yang sudah tidak berlaku.
  * Menjalankan kembali logika alokasi dana baru secara kronologis ke daftar invoice terupdate.
