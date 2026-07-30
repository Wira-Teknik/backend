# Panduan Alur Teknis Sistem (System Flow) - Wira Teknik

Dokumen ini menjelaskan arsitektur teknis, desain database, logika bisnis internal, dan alur transisi status dalam sistem backend **Wira Teknik**.

---

## 1. Arsitektur & Teknologi

* **Runtime & Framework**: Go (Golang) menggunakan framework **Fiber v2** untuk HTTP Server.
* **Database & ORM**: PostgreSQL sebagai database relasional, diakses menggunakan **GORM**.
* **Keamanan & Otorisasi**:
  * Autentikasi berbasis JWT Token yang disimpan di cookie `session_token` atau dikirim via header `Authorization: Bearer <token>`.
  * Middleware `RequireAuth` mengekstrak klaim pengguna (ID, Email, Role).
  * Middleware `RequireRole` membatasi akses endpoint tertentu (misalnya menu `/audit` hanya dapat diakses oleh role `owner`).
* **Sistem Latar Belakang (Asynchronous Tasks)**:
  * Pengiriman notifikasi email diproses secara asinkron menggunakan goroutine (`go func() ...`) untuk mencegah pemblokiran HTTP response thread.

---

## 2. Skema Relasi Database & Detail GORM Model

Sistem ini menggunakan UUID v4 (`gen_random_uuid()`) sebagai Primary Key pada seluruh tabel untuk skalabilitas dan keamanan ID.

### A. Entitas Pengguna & Hak Akses
1. **`users`**
   * `id`: `uuid` (Primary Key, default: `gen_random_uuid()`)
   * `email`: `varchar(255)` (Unique Index, Not Null)
   * `password_hash`: `text` (Not Null)
   * `role`: `varchar(20)` (Not Null, check: `role IN ('admin', 'owner')`)
   * `created_at` & `updated_at`: `timestamp`
   * `deleted_at`: `gorm.DeletedAt` (Soft delete index)

### B. Entitas Bisnis Utama
1. **`customers`**
   * `id`: `uuid` (PK)
   * `name`: `varchar(255)` (Unique Index, Not Null)
   * `address`: `text`
   * `phone` & `email`: `varchar`

2. **`orders`**
   * `id`: `uuid` (PK)
   * `customer_id`: `uuid` (Foreign Key -> `customers.id`)
   * `transaction_no`: `varchar(255)` (Unique Index, format: `NF/WT/{running_number}/{year}`)
   * `po_no`: `varchar(255)` (Nomor Purchase Order dari pembeli)
   * `order_date`: `timestamp` (Not Null)
   * `order_status`: `varchar(20)` (check: `order_status IN ('pending', 'partial', 'shipped', 'completed')`)
   * Relasi: Has Many `order_items`, Has Many `shipments`

3. **`order_items`**
   * `id`: `uuid` (PK)
   * `order_id`: `uuid` (FK -> `orders.id` dengan Cascade Delete)
   * `product_name`: `varchar(255)` (Not Null)
   * `order_qty`: `integer` (Kuantitas pesanan awal)
   * `remaining_qty`: `integer` (Sisa kuantitas barang yang belum dikirimkan)
   * `unit_price`: `double precision`
   * `created_at` & `updated_at`: `timestamp`

4. **`shipments`**
   * `id`: `uuid` (PK)
   * `order_id`: `uuid` (FK -> `orders.id` dengan Cascade Delete)
   * `shipping_date`: `timestamp` (Not Null)
   * `received_date`: `timestamp` (Null jika belum diterima)
   * `shipping_status`: `varchar(20)` (check: `shipping_status IN ('dikirim', 'diterima')`)
   * Relasi: Has Many `shipment_items`, Has One `invoice`

5. **`shipment_items`**
   * `id`: `uuid` (PK)
   * `shipment_id`: `uuid` (FK -> `shipments.id` dengan Cascade Delete)
   * `order_item_id`: `uuid` (FK -> `order_items.id`)
   * `shipping_qty`: `integer` (Kuantitas yang dikirim dalam pengiriman ini)

6. **`invoices`**
   * `id`: `uuid` (PK)
   * `shipment_id`: `uuid` (FK -> `shipments.id` dengan Cascade Delete)
   * `invoice_no`: `varchar(255)` (Unique Index, format: `INV-YYYYMMDD-XXXX`)
   * `total_amount`: `double precision` (Subtotal item dikirim + PPN 11%)
   * `remaining_balance`: `double precision` (Sisa tagihan yang harus dibayar)
   * `payment_status`: `varchar(20)` (check: `payment_status IN ('unpaid', 'partial', 'paid')`)

7. **`payments`**
   * `id`: `uuid` (PK)
   * `payment_total`: `double precision` (Total dana yang dibayarkan)
   * `payment_date`: `timestamp` (Not Null)
   * Relasi: Has Many `payment_details`

8. **`payment_details`**
   * `id`: `uuid` (PK)
   * `payment_id`: `uuid` (FK -> `payments.id` dengan Cascade Delete)
   * `invoice_id`: `uuid` (FK -> `invoices.id` dengan Cascade Delete)
   * `allocated_amount`: `double precision` (Porsi pembayaran yang dialokasikan untuk invoice terkait)

9. **`attachments`**
   * `id`: `uuid` (PK)
   * `related_id`: `uuid` (Bisa berupa `shipment_id` atau `payment_id`)
   * `file_path`: `text` (Lokasi penyimpanan file di server)
   * `file_type`: `varchar(50)` (Contoh: `surat_jalan`, `bukti_kirim`, `bon`, `payment_proof`)

---

## 3. Logika Transaksi & Validasi Bisnis Internal

Sistem membungkus setiap operasi mutasi data menggunakan **Database Transaction (Atomik)** untuk menjamin konsistensi data. Jika salah satu langkah gagal, seluruh perubahan di-rollback (`tx.Rollback()`).

### A. Siklus Hidup Pesanan (Order Status Engine)
Status `orders.order_status` dihitung ulang secara dinamis menggunakan fungsi internal `updateOrderStatus` setelah terjadi perubahan item pengiriman:
* **`pending`**: Kondisi awal. Tidak ada item yang terkirim (`order_items.remaining_qty == order_items.order_qty` untuk semua item).
* **`partial`**: Ada sebagian item yang sudah dikirim (`remaining_qty < order_qty` untuk minimal satu item, namun masih ada item yang `remaining_qty > 0`).
* **`shipped`**: Semua item pesanan sudah dikirim sepenuhnya (`remaining_qty == 0` untuk seluruh item).
* **`completed`**: Seluruh pengiriman (`shipments`) terkait pesanan tersebut telah terkonfirmasi diterima (`shipping_status == 'diterima'`) **DAN** status pengiriman barang sudah mencapai `shipped`.

### B. Validasi Pengiriman (`POST /api/v1/shipments`)
1. **Pengecekan Tanggal**:
   * Sistem membaca `order_date` dari pesanan terkait.
   * `shipping_date` yang dikirim dari klien diparsing menggunakan format `YYYY-MM-DD`.
   * **Validasi**: Jika `shipping_date` < `order_date`, sistem langsung menghentikan proses dan mengembalikan kode respon HTTP 400 (`ErrShipmentDateBeforeOrderDate`).
2. **Pengecekan Kuantitas**:
   * Untuk setiap item yang akan dikirim, sistem mengecek sisa kuantitas pesanan yang tersimpan di `order_items.remaining_qty`.
   * **Validasi**: Jika `shipping_qty` > `remaining_qty`, sistem membatalkan transaksi untuk menghindari pengiriman barang melebihi kuantitas pesanan.
3. **Penyimpanan Data & Pembaruan Stok**:
   * Membuat record `shipment` baru.
   * Mengurangi `order_items.remaining_qty` secara atomik di database:
     ```sql
     UPDATE order_items SET remaining_qty = remaining_qty - ? WHERE id = ?
     ```
   * Memanggil `updateOrderStatus` untuk mengevaluasi status order baru.

### C. Pembuatan Invoice Otomatis
Setelah data pengiriman disimpan secara valid dalam transaksi database yang sama:
1. Nominal PPN didefinisikan sebesar 11% (`0.11`).
2. **Kalkulasi Total Tagihan**:
   $$\text{Subtotal} = \sum (\text{shipment\_item.shipping\_qty} \times \text{order\_item.unit\_price})$$
   $$\text{Total Amount} = \text{Subtotal} \times 1.11$$
   * Nominal dibulatkan ke dua angka desimal (`roundTwo`).
3. **Penomoran Invoice**: Nomor invoice dibuat secara berurutan per hari dengan format `INV-YYYYMMDD-XXXX`.
   * Sistem melakukan query `COUNT` pada invoice berawalan `INV-YYYYMMDD-` di hari berjalan untuk menentukan urutan berikutnya (`XXXX`).
4. Menyimpan record `invoice` dengan status awal `unpaid` dan sisa tagihan `remaining_balance = total_amount`.

### D. Konfirmasi Penerimaan Pengiriman (`PATCH /api/v1/shipments/:id/received`)
1. Menandai `shipping_status = 'diterima'` dan mengisi `received_date` dengan waktu server berjalan.
2. Memeriksa sisa pengiriman yang masih berstatus `dikirim` untuk pesanan tersebut.
3. Jika tidak ada lagi pengiriman yang berstatus `dikirim` (semua sudah `diterima`) **DAN** semua item pesanan sudah habis dikirim (`remaining_qty == 0`), maka status pesanan diperbarui menjadi `completed`.

---

## 4. Algoritma Pembayaran (Payment Allocation & Reversion)

### A. Alokasi Pembayaran (`CreatePayment`)
Ketika pembayaran baru dicatat untuk daftar pesanan terpilih (`order_ids`):
1. **Query Tagihan**: Mencari semua invoice dari pesanan terkait yang kolom `payment_status` tidak sama dengan `paid`.
2. **Pecahan Urutan**: Tagihan diurutkan berdasarkan `invoices.created_at ASC` agar tagihan terlama dilunasi terlebih dahulu (Metode FIFO).
3. **Validasi Kapasitas**: Total nominal pembayaran (`payment_total`) tidak boleh melebihi sisa kumulatif tagihan dari invoice yang ditemukan.
4. **Proses Loop Alokasi**:
   * Sistem membawa variabel penampung sisa dana `sisaPembayaran = payment_total`.
   * Untuk setiap invoice:
     * Jika `sisaPembayaran <= 0`, loop berhenti.
     * Jika `sisaPembayaran >= invoice.remaining_balance`:
       * Nominal alokasi = `invoice.remaining_balance`.
       * Nilai sisa tagihan invoice diset = `0`, status = `paid`.
       * Kurangi sisa dana: `sisaPembayaran = sisaPembayaran - nominal alokasi`.
     * Jika `sisaPembayaran < invoice.remaining_balance`:
       * Nominal alokasi = `sisaPembayaran`.
       * Nilai sisa tagihan invoice dikurangi sebesar `sisaPembayaran`, status = `partial`.
       * Sisa dana diset = `0`.
     * Catat alokasi tersebut ke dalam tabel `payment_details`.

### B. Pembatalan Alokasi & Alokasi Ulang (`UpdatePaymentTotal` / Edit)
Proses edit pembayaran wajib memulihkan keadaan pembukuan sebelum menerapkan nilai pembayaran baru. Alur teknisnya:
1. **Revert (Rollback Keadaan)**:
   * Mengambil semua alokasi pembayaran lama dari tabel `payment_details` berdasarkan `payment_id` yang diubah.
   * Untuk setiap detail alokasi:
     * Mengembalikan nominal saldo tagihan invoice:
       $$\text{invoice.remaining\_balance} = \text{invoice.remaining\_balance} + \text{detail.allocated\_amount}$$
     * Memperbarui kembali status pembayaran invoice: Jika `remaining_balance` sama dengan total awal invoice, status kembali menjadi `unpaid`, jika kurang maka status `partial`.
   * Menghapus seluruh baris alokasi lama di `payment_details`.
2. **Re-Allocation**:
   * Menerapkan nominal pembayaran baru (`payment_total`) dan daftar pesanan baru (`order_ids`) menggunakan Algoritma Alokasi Pembayaran (Bagian A).

---

## 5. Audit Logging & Lini Masa Dashboard

1. **Audit Logs**:
   * Setiap aksi sensitif (penulisan/pembaruan/penghapusan) pada tabel `orders`, `shipments`, `invoices`, dan `payments` memanggil helper `CreateAuditLog`.
   * Menyimpan JSON string berisi data lama (*before*) dan data baru (*after*) untuk kemudahan audit log.
2. **Timeline Dashboard sekuensial**:
   * Ketika shipment dibuat, log mencatat `Pengiriman Terkonfirmasi` (status: dikirim).
   * Ketika konfirmasi penerimaan dipanggil, log mencatat entri baru `Pengiriman Selesai` (status: diterima) tanpa menimpa entri log kirim sebelumnya. Hal ini menjamin urutan kronologis aktivitas pada UI dashboard tetap konsisten dan berurutan.
