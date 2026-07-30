# Panduan Pengujian Logika Bisnis (Unit Testing) - Wira Teknik

Dokumen ini berisi panduan untuk melakukan pengujian otomatis terhadap aturan bisnis (*business rules*) utama sistem backend Wira Teknik. Pengujian ini sangat penting untuk dipresentasikan saat **Sidang Skripsi** guna membuktikan kebenaran logika pemrograman (*correctness*) secara ilmiah dan empiris di hadapan dosen penguji.

---

## 1. Fitur Utama yang Diuji & Skenario Kasus

Skenario pengujian berfokus pada aturan validasi ketat, integritas transaksional, dan mesin status (*state machine*) yang telah kita implementasikan:

| No | Fitur / Kasus Uji | Skenario Uji | Ekspektasi Hasil (Assertion) |
|---|---|---|---|
| **1** | **Validasi Tanggal Pengiriman** | Membuat pengiriman dengan tanggal (`shipping_date`) sebelum tanggal pesanan dibuat (`order_date`). | Transaksi ditolak, sistem mengembalikan error `ErrShipmentDateBeforeOrderDate`. |
| **2** | **Batas Kuantitas Pengiriman** | Mengirimkan barang dengan kuantitas (`shipping_qty`) melebihi sisa barang yang dipesan (`remaining_qty`). | Transaksi ditolak, stok tidak bocor/negatif. |
| **3** | **Mesin Status Pesanan** | Melakukan pengiriman secara parsial, lalu mengirimkan sisa barang, dilanjutkan dengan konfirmasi penerimaan barang. | Status pesanan otomatis berpindah: `pending` $\rightarrow$ `partial` $\rightarrow$ `shipped` $\rightarrow$ `completed`. |
| **4** | **Alokasi Pembayaran FIFO** | Membayar tagihan dari pesanan yang memiliki beberapa invoice dengan nominal pembayaran yang tidak penuh. | Pembayaran secara otomatis terbagi melunasi invoice terlama terlebih dahulu (FIFO). |
| **5** | **Rollback saat Edit Pembayaran** | Mengubah nominal pembayaran yang sudah dialokasikan ke beberapa invoice sebelumnya. | Nominal alokasi lama dihapus, saldo sisa invoice lama dipulihkan ke keadaan semula (*rollback*), lalu dana dialokasikan ulang berdasarkan input baru. |
| **6** | **Pengingat Jatuh Tempo** | Membuat pesanan berumur > 3 bulan (misal 4 bulan) yang belum lunas. | Memicu `Send3MonthOverduePaymentReminders()`, email pengingat terkirim ke pelanggan. |

---

## 2. Struktur Kode Script Test

Kita telah membuat berkas pengujian otomatis di [tests/business_logic_test.go] dan [tests/payment_reminder_test.go]. Script ini memanfaatkan:
* **SQLite In-Memory / PostgreSQL Sandboxed Transaction**: Setiap fungsi tes diawali dengan `tx := config.DB.Begin()` dan diakhiri dengan `defer tx.Rollback()`. Ini memastikan database asli tidak kotor oleh data uji coba.
* **TDD / Go Testing Tool**: Menggunakan package standar bawaan Go `testing`.

---

## 3. Cara Menjalankan Pengujian

Jalankan perintah berikut di terminal proyek untuk memulai pengujian otomatis:

```powershell
# Jalankan seluruh unit test di folder tests dengan output verbose
go test -v ./tests/...
```

Hasil eksekusi yang sukses akan memunculkan tulisan `PASS` untuk masing-masing skenario pengujian di atas. Dokumentasi dan hasil terminal ini dapat dicantumkan pada Bab 4 (Hasil dan Pembahasan) di buku skripsi Anda.
