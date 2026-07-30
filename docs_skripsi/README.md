# Dokumentasi Logika Bisnis Aplikasi - Persiapan Sidang Skripsi

Folder ini berisi penjelasan terperinci mengenai fungsi, arsitektur logika, flowchart, dan poin-poin teknis penting dari setiap berkas layanan (*service*) di dalam sistem backend **Wira Teknik**. Dokumentasi ini dirancang menggunakan bahasa Indonesia yang akademis dan mudah dipahami untuk membantu presentasi **Sidang Skripsi**.

## Daftar Berkas Layanan (Services)

Silakan klik tautan di bawah ini untuk melihat penjelasan masing-masing layanan:

1. **[Auth Service](file:///c:/Users/apany/Documents/GitHub/golang/teknik/docs_skripsi/auth_service.md)** - Mengelola pendaftaran akun pengguna, masuk sistem, token stateless JWT, dan fitur lupa kata sandi via OTP Gmail + Redis.
2. **[Customer Service](file:///c:/Users/apany/Documents/GitHub/golang/teknik/docs_skripsi/customer_service.md)** - Mengelola pendaftaran profil perusahaan/pelanggan serta penerapan pengamanan data menggunakan *Soft Delete*.
3. **[Order Service](file:///c:/Users/apany/Documents/GitHub/golang/teknik/docs_skripsi/order_service.md)** - Mengelola siklus hidup pesanan pembelian (*Purchase Order*), penomoran nomor transaksi otomatis tahunan, serta validasi penghapusan data berstatus pending.
4. **[Shipment Service](file:///c:/Users/apany/Documents/GitHub/golang/teknik/docs_skripsi/shipment_service.md)** - Mengatur logika pengiriman barang parsial/penuh, validasi kuantitas kirim vs pesanan, dan konfirmasi barang tiba.
5. **[Invoice Service](file:///c:/Users/apany/Documents/GitHub/golang/teknik/docs_skripsi/invoice_service.md)** - Mengelola data tagihan otomatis yang terbit setelah pengiriman barang beserta status pelunasannya.
6. **[Payment Service](file:///c:/Users/apany/Documents/GitHub/golang/teknik/docs_skripsi/payment_service.md)** - Mengelola pencatatan pembayaran menggunakan algoritma alokasi tagihan terlama (FIFO) serta logika pemulihan (*rollback*) saat pembayaran diubah.
7. **[Attachment Service](file:///c:/Users/apany/Documents/GitHub/golang/teknik/docs_skripsi/attachment_service.md)** - Mengatur pengunggahan berkas bukti pendukung fisik (surat jalan, bukti transfer) dengan sanitasi nama berkas dan relasi polimorfisme.
8. **[Email Service](file:///c:/Users/apany/Documents/GitHub/golang/teknik/docs_skripsi/email_service.md)** - Mengelola pengiriman email template HTML otomatis secara asinkron menggunakan protokol SMTP Gmail.
9. **[Dashboard Service](file:///c:/Users/apany/Documents/GitHub/golang/teknik/docs_skripsi/dashboard_service.md)** - Mengagregasikan statistik metrik operasional utama dan lini masa kejadian logistik secara sekuensial.
10. **[Payment Recap Service](file:///c:/Users/apany/Documents/GitHub/golang/teknik/docs_skripsi/payment_recap_service.md)** - Mengelola rekapitulasi pelaporan keuangan tahunan/bulanan serta fitur ekspor data otomatis ke file spreadsheet Excel (`.xlsx`).
11. **[Audit Service](file:///c:/Users/apany/Documents/GitHub/golang/teknik/docs_skripsi/audit_service.md)** - Merekam riwayat mutasi data sensitif (*before-after data snapshot*) yang khusus disajikan bagi akun pemilik (*owner*).
