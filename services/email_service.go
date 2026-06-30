package services

import (
	"encoding/base64"
	"fmt"
	"net/smtp"
	"os"
	"path/filepath"
	"strings"
	"time"

	"teknik/config"
	"teknik/models"

	"github.com/google/uuid"
)

// formatIDR formats monetary values into Indonesian Rupiah format.
func formatIDR(amount float64) string {
	parts := strings.Split(fmt.Sprintf("%.2f", amount), ".")
	intPart := parts[0]
	decPart := parts[1]

	var result []string
	length := len(intPart)
	for i := length; i > 0; i -= 3 {
		start := i - 3
		if start < 0 {
			start = 0
		}
		result = append([]string{intPart[start:i]}, result...)
	}
	return "Rp " + strings.Join(result, ".") + "," + decPart
}

// formatDateIndo formats time.Time to readable Indonesian date format.
func formatDateIndo(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	months := []string{
		"Januari", "Februari", "Maret", "April", "Mei", "Juni",
		"Juli", "Agustus", "September", "Oktober", "November", "Desember",
	}
	monthIdx := int(t.Month()) - 1
	if monthIdx < 0 || monthIdx > 11 {
		monthIdx = 0
	}
	return fmt.Sprintf("%02d %s %d", t.Day(), months[monthIdx], t.Year())
}

// getEmailHeader returns a styled HTML header for email notifications.
func getEmailHeader(title string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>%s</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
            background-color: #f3f4f6;
            margin: 0;
            padding: 0;
            -webkit-font-smoothing: antialiased;
        }
        .container {
            max-width: 600px;
            margin: 40px auto;
            background: #ffffff;
            border-radius: 12px;
            overflow: hidden;
            box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.1), 0 2px 4px -1px rgba(0, 0, 0, 0.06);
            border: 1px solid #e5e7eb;
        }
        .header {
            background: linear-gradient(135deg, #1e3a8a 0%%, #3b82f6 100%%);
            color: #ffffff;
            padding: 32px 24px;
            text-align: center;
        }
        .header h1 {
            margin: 0;
            font-size: 24px;
            font-weight: 700;
            letter-spacing: -0.025em;
        }
        .content {
            padding: 32px 24px;
            color: #374151;
            line-height: 1.6;
        }
        .content p {
            margin-top: 0;
            margin-bottom: 16px;
        }
        .details-table {
            width: 100%%;
            border-collapse: collapse;
            margin: 24px 0;
            font-size: 14px;
        }
        .details-table th {
            background-color: #f8fafc;
            color: #475569;
            font-weight: 600;
            text-align: left;
            padding: 12px;
            border-bottom: 2px solid #e2e8f0;
        }
        .details-table td {
            padding: 12px;
            border-bottom: 1px solid #e2e8f0;
            color: #334155;
        }
        .details-table tr:last-child td {
            border-bottom: none;
        }
        .badge {
            display: inline-block;
            padding: 6px 12px;
            font-size: 12px;
            font-weight: 600;
            border-radius: 9999px;
            text-transform: uppercase;
        }
        .badge-success {
            background-color: #dcfce7;
            color: #166534;
        }
        .badge-info {
            background-color: #dbeafe;
            color: #1e40af;
        }
        .footer {
            background-color: #f9fafb;
            padding: 24px;
            text-align: center;
            font-size: 12px;
            color: #6b7280;
            border-top: 1px solid #e5e7eb;
        }
        .footer p {
            margin: 4px 0;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>%s</h1>
        </div>
        <div class="content">`, title, title)
}

// getEmailFooter returns the HTML footer.
func getEmailFooter() string {
	appName := os.Getenv("APP_NAME")
	if appName == "" {
		appName = "Wira Teknik"
	}
	return fmt.Sprintf(`        </div>
        <div class="footer">
            <p>&copy; %d %s. All rights reserved.</p>
            <p>Email ini dikirim secara otomatis oleh sistem. Jangan membalas email ini.</p>
        </div>
    </div>
</body>
</html>`, time.Now().Year(), appName)
}

// SendEmail sends a basic HTML email.
func SendEmail(toEmail, subject, htmlBody string) error {
	smtpHost := os.Getenv("SMTP_HOST")
	smtpPort := os.Getenv("SMTP_PORT")
	smtpUser := os.Getenv("SMTP_USER")
	smtpPass := os.Getenv("SMTP_PASS")
	fromName := os.Getenv("APP_NAME")
	if fromName == "" {
		fromName = "Wira Teknik"
	}

	header := make(map[string]string)
	header["From"] = fmt.Sprintf("%s <%s>", fromName, smtpUser)
	header["To"] = toEmail
	header["Subject"] = subject
	header["MIME-Version"] = "1.0"
	header["Content-Type"] = "text/html; charset=UTF-8"

	var message strings.Builder
	for k, v := range header {
		message.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	message.WriteString("\r\n")
	message.WriteString(htmlBody)

	auth := smtp.PlainAuth("", smtpUser, smtpPass, smtpHost)
	addr := fmt.Sprintf("%s:%s", smtpHost, smtpPort)

	return smtp.SendMail(addr, auth, smtpUser, []string{toEmail}, []byte(message.String()))
}

// SendEmailWithAttachment sends an HTML email with a PDF file attached.
func SendEmailWithAttachment(toEmail, subject, htmlBody, filename string, attachmentBytes []byte) error {
	smtpHost := os.Getenv("SMTP_HOST")
	smtpPort := os.Getenv("SMTP_PORT")
	smtpUser := os.Getenv("SMTP_USER")
	smtpPass := os.Getenv("SMTP_PASS")
	fromName := os.Getenv("APP_NAME")
	if fromName == "" {
		fromName = "Wira Teknik"
	}

	boundary := "wira-teknik-email-boundary-987654321"

	// Headers
	header := make(map[string]string)
	header["From"] = fmt.Sprintf("%s <%s>", fromName, smtpUser)
	header["To"] = toEmail
	header["Subject"] = subject
	header["MIME-Version"] = "1.0"
	header["Content-Type"] = fmt.Sprintf("multipart/mixed; boundary=%s", boundary)

	var message strings.Builder
	for k, v := range header {
		message.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	message.WriteString("\r\n")

	// HTML part
	message.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	message.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	message.WriteString("Content-Transfer-Encoding: 7bit\r\n\r\n")
	message.WriteString(htmlBody)
	message.WriteString("\r\n\r\n")

	// Attachment part
	if len(attachmentBytes) > 0 {
		message.WriteString(fmt.Sprintf("--%s\r\n", boundary))
		message.WriteString(fmt.Sprintf("Content-Type: application/pdf; name=\"%s\"\r\n", filename))
		message.WriteString("Content-Transfer-Encoding: base64\r\n")
		message.WriteString(fmt.Sprintf("Content-Disposition: attachment; filename=\"%s\"\r\n\r\n", filename))

		// base64 encode attachment
		b64Content := base64.StdEncoding.EncodeToString(attachmentBytes)
		for i := 0; i < len(b64Content); i += 76 {
			end := i + 76
			if end > len(b64Content) {
				end = len(b64Content)
			}
			message.WriteString(b64Content[i:end])
			message.WriteString("\r\n")
		}
		message.WriteString("\r\n")
	}

	message.WriteString(fmt.Sprintf("--%s--\r\n", boundary))

	auth := smtp.PlainAuth("", smtpUser, smtpPass, smtpHost)
	addr := fmt.Sprintf("%s:%s", smtpHost, smtpPort)

	return smtp.SendMail(addr, auth, smtpUser, []string{toEmail}, []byte(message.String()))
}

// GetUploadedInvoiceAttachment finds and reads the uploaded invoice document associated with the Invoice ID or Shipment ID.
func GetUploadedInvoiceAttachment(invoiceID uuid.UUID) (string, []byte, error) {
	var attachment models.Attachment

	// 1. Try finding by Invoice ID
	err := config.DB.Where("related_id = ? AND category = ?", invoiceID, models.AttachmentCategoryInvoice).First(&attachment).Error
	if err == nil {
		filePath := strings.TrimPrefix(attachment.FileURL, "/")
		data, err := os.ReadFile(filePath)
		if err == nil {
			return filepath.Base(filePath), data, nil
		}
	}

	// 2. If not found, look up the invoice to get ShipmentID, then find attachment for ShipmentID
	var invoice models.Invoice
	if err := config.DB.First(&invoice, "id = ?", invoiceID).Error; err == nil {
		var shipmentAttachment models.Attachment
		err = config.DB.Where("related_id = ? AND category = ?", invoice.ShipmentID, models.AttachmentCategoryInvoice).First(&shipmentAttachment).Error
		if err == nil {
			filePath := strings.TrimPrefix(shipmentAttachment.FileURL, "/")
			data, err := os.ReadFile(filePath)
			if err == nil {
				return filepath.Base(filePath), data, nil
			}
		}
	}

	return "", nil, fmt.Errorf("dokumen invoice yang diupload tidak ditemukan di database atau disk")
}

// SendShipmentNotificationEmail triggers an email when a shipment is dispatched (Dikirim).
func SendShipmentNotificationEmail(shipmentID uuid.UUID) error {
	var shipment models.Shipment
	err := config.DB.
		Preload("Order").
		Preload("Items").
		Preload("Items.OrderItem").
		First(&shipment, "id = ?", shipmentID).Error
	if err != nil {
		return err
	}

	if shipment.Order.RecipientEmail == "" {
		fmt.Printf("Warning: Recipient email is empty for shipment %s, skipping notification email.\n", shipment.ID)
		return nil
	}

	var itemsTable strings.Builder
	itemsTable.WriteString(`<table class="details-table">
        <thead>
            <tr>
                <th>No</th>
                <th>Nama Produk</th>
                <th>Jumlah Dikirim</th>
            </tr>
        </thead>
        <tbody>`)
	for idx, item := range shipment.Items {
		itemsTable.WriteString(fmt.Sprintf(`
            <tr>
                <td>%d</td>
                <td>%s</td>
                <td>%d Pcs</td>
            </tr>`, idx+1, item.OrderItem.ProductName, item.ShippingQty))
	}
	itemsTable.WriteString(`</tbody>
    </table>`)

	body := fmt.Sprintf(`
        <p>Halo <strong>%s</strong>,</p>
        <p>Kabar baik! Pesanan Anda dengan nomor transaksi <strong>%s</strong> sedang dalam proses pengiriman oleh kurir kami.</p>
        <p>Detail pengiriman:</p>
        <ul>
            <li>Tanggal Kirim: <strong>%s</strong></li>
            <li>Status Pengiriman: <span class="badge badge-info">%s</span></li>
        </ul>
        <p>Berikut adalah daftar barang yang sedang dikirim:</p>
        %s
        <p>Kami akan mengirimkan notifikasi dan dokumen invoice resmi setelah barang sampai di lokasi Anda.</p>
        <p>Terima kasih telah berbelanja di Wira Teknik.</p>
        <p>Salam hangat,<br>Tim Wira Teknik</p>`,
		shipment.Order.RecipientName,
		shipment.Order.TransactionNo,
		formatDateIndo(time.Time(shipment.ShippingDate)),
		strings.Title(string(shipment.ShippingStatus)),
		itemsTable.String(),
	)

	htmlContent := getEmailHeader("Notifikasi Pengiriman Pesanan") + body + getEmailFooter()
	subject := fmt.Sprintf("Notifikasi Pengiriman Pesanan - %s", shipment.Order.TransactionNo)

	return SendEmail(shipment.Order.RecipientEmail, subject, htmlContent)
}

// SendShipmentReceivedNotificationEmail triggers an email when a shipment is confirmed received.
// It retrieves the uploaded PDF Invoice from disk.
func SendShipmentReceivedNotificationEmail(shipmentID uuid.UUID) error {
	var shipment models.Shipment
	err := config.DB.
		Preload("Order").
		Preload("Invoice").
		Preload("Items").
		Preload("Items.OrderItem").
		First(&shipment, "id = ?", shipmentID).Error
	if err != nil {
		return err
	}

	if shipment.Order.RecipientEmail == "" {
		fmt.Printf("Warning: Recipient email is empty for shipment %s, skipping received email.\n", shipment.ID)
		return nil
	}

	if shipment.Invoice == nil {
		return fmt.Errorf("invoice not found for shipment %s", shipment.ID)
	}

	// Fetch the uploaded Invoice PDF
	filename, pdfBytes, err := GetUploadedInvoiceAttachment(shipment.Invoice.ID)
	if err != nil {
		// Log warning and fallback to sending the email without attachment if it's missing on disk/db
		fmt.Printf("Warning: failed to get uploaded invoice attachment for invoice %s: %v. Sending email without attachment.\n", shipment.Invoice.ID, err)
		filename = ""
		pdfBytes = nil
	}

	var itemsTable strings.Builder
	itemsTable.WriteString(`<table class="details-table">
        <thead>
            <tr>
                <th>No</th>
                <th>Nama Produk</th>
                <th>Jumlah Diterima</th>
            </tr>
        </thead>
        <tbody>`)
	for idx, item := range shipment.Items {
		itemsTable.WriteString(fmt.Sprintf(`
            <tr>
                <td>%d</td>
                <td>%s</td>
                <td>%d Pcs</td>
            </tr>`, idx+1, item.OrderItem.ProductName, item.ShippingQty))
	}
	itemsTable.WriteString(`</tbody>
    </table>`)

	receivedDateStr := "-"
	if shipment.ReceivedDate != nil {
		receivedDateStr = formatDateIndo(time.Time(*shipment.ReceivedDate))
	} else {
		receivedDateStr = formatDateIndo(time.Now())
	}

	attachmentText := ""
	if len(pdfBytes) > 0 {
		attachmentText = fmt.Sprintf("<p>Terlampir kami sertakan dokumen tagihan (invoice) resmi <strong>%s</strong> dengan nominal total <strong>%s</strong> yang diunggah untuk pengiriman ini.</p>", shipment.Invoice.InvoiceNo, formatIDR(shipment.Invoice.TotalAmount))
	} else {
		attachmentText = fmt.Sprintf("<p>Nominal total tagihan untuk pengiriman ini adalah <strong>%s</strong> (nomor invoice: <strong>%s</strong>).</p>", formatIDR(shipment.Invoice.TotalAmount), shipment.Invoice.InvoiceNo)
	}

	body := fmt.Sprintf(`
        <p>Halo <strong>%s</strong>,</p>
        <p>Kami mengonfirmasi bahwa barang pengiriman untuk pesanan Anda dengan nomor transaksi <strong>%s</strong> telah diterima dengan baik.</p>
        <p>Detail penerimaan barang:</p>
        <ul>
            <li>Tanggal Diterima: <strong>%s</strong></li>
            <li>Status Pengiriman: <span class="badge badge-success">%s</span></li>
        </ul>
        <p>Berikut adalah rincian barang yang telah Anda terima:</p>
        %s
        %s
        <p>Mohon melakukan pembayaran sesuai dengan rincian yang tertera pada invoice Anda.</p>
        <p>Terima kasih atas kepercayaan Anda menggunakan layanan Wira Teknik.</p>
        <p>Salam hangat,<br>Tim Wira Teknik</p>`,
		shipment.Order.RecipientName,
		shipment.Order.TransactionNo,
		receivedDateStr,
		strings.Title(string(shipment.ShippingStatus)),
		itemsTable.String(),
		attachmentText,
	)

	htmlContent := getEmailHeader("Konfirmasi Penerimaan Barang & Tagihan") + body + getEmailFooter()
	subject := fmt.Sprintf("Konfirmasi Penerimaan Barang & Invoice - %s", shipment.Invoice.InvoiceNo)

	if len(pdfBytes) > 0 {
		return SendEmailWithAttachment(shipment.Order.RecipientEmail, subject, htmlContent, filename, pdfBytes)
	}
	return SendEmail(shipment.Order.RecipientEmail, subject, htmlContent)
}

// SendInvoicePaidNotificationEmail triggers an email when an invoice status becomes PAID (Lunas).
func SendInvoicePaidNotificationEmail(invoiceID uuid.UUID) error {
	var invoice models.Invoice
	err := config.DB.
		Preload("Shipment").
		Preload("Shipment.Order").
		First(&invoice, "id = ?", invoiceID).Error
	if err != nil {
		return err
	}

	recipientEmail := invoice.Shipment.Order.RecipientEmail
	recipientName := invoice.Shipment.Order.RecipientName
	if recipientEmail == "" {
		fmt.Printf("Warning: Recipient email is empty for invoice %s, skipping paid notification.\n", invoice.ID)
		return nil
	}

	body := fmt.Sprintf(`
        <p>Halo <strong>%s</strong>,</p>
        <p>Terima kasih atas pembayaran Anda.</p>
        <p>Kami ingin memberitahukan bahwa tagihan (invoice) dengan nomor <strong>%s</strong> untuk pesanan <strong>%s</strong> telah kami terima pembayarannya secara penuh dan dinyatakan <strong>LUNAS</strong>.</p>
        <p>Detail pembayaran:</p>
        <table class="details-table">
            <thead>
                <tr>
                    <th>Detail Tagihan</th>
                    <th>Keterangan</th>
                </tr>
            </thead>
            <tbody>
                <tr>
                    <td>Nomor Invoice</td>
                    <td><strong>%s</strong></td>
                </tr>
                <tr>
                    <td>Nomor Transaksi</td>
                    <td>%s</td>
                </tr>
                <tr>
                    <td>Total Tagihan</td>
                    <td><strong>%s</strong></td>
                </tr>
                <tr>
                    <td>Status Pembayaran</td>
                    <td><span class="badge badge-success">Lunas / Paid</span></td>
                </tr>
            </tbody>
        </table>
        <p>Terima kasih atas kerja sama dan kepercayaan Anda kepada kami.</p>
        <p>Salam hangat,<br>Tim Wira Teknik</p>`,
		recipientName,
		invoice.InvoiceNo,
		invoice.Shipment.Order.TransactionNo,
		invoice.InvoiceNo,
		invoice.Shipment.Order.TransactionNo,
		formatIDR(invoice.TotalAmount),
	)

	htmlContent := getEmailHeader("Notifikasi Pembayaran Lunas") + body + getEmailFooter()
	subject := fmt.Sprintf("Pembayaran Lunas - Invoice %s", invoice.InvoiceNo)

	return SendEmail(recipientEmail, subject, htmlContent)
}

// SendPaymentDueReminderEmail sends a notification for payment due 3 months after order creation.
func SendPaymentDueReminderEmail(orderID uuid.UUID) error {
	var order models.Order
	err := config.DB.
		Preload("Items").
		Preload("Shipments").
		Preload("Shipments.Invoice").
		First(&order, "id = ?", orderID).Error
	if err != nil {
		return err
	}

	// Calculate payment details
	computeOrderPaymentInfo(&order)

	// If the order is already fully paid, no reminder is needed
	if order.PaymentStatus == models.PaymentStatusPaid || order.RemainingBalance <= 0 {
		return nil
	}

	if order.RecipientEmail == "" {
		fmt.Printf("Warning: Recipient email is empty for order %s, skipping due reminder.\n", order.ID)
		return nil
	}

	orderDate := time.Time(order.OrderDate)
	dueDate := orderDate.AddDate(0, 3, 0) // 3 months since order creation

	body := fmt.Sprintf(`
        <p>Halo <strong>%s</strong>,</p>
        <p>Kami ingin menginfokan bahwa pembayaran untuk pesanan Anda dengan nomor transaksi <strong>%s</strong> (PO: <strong>%s</strong>) telah memasuki batas waktu jatuh tempo.</p>
        <p>Sesuai dengan ketentuan termin pembayaran kami, batas waktu pelunasan adalah <strong>3 bulan sejak tanggal pesanan dibuat</strong>.</p>
        <p>Detail tagihan Anda:</p>
        <table class="details-table">
            <thead>
                <tr>
                    <th>Detail Transaksi</th>
                    <th>Keterangan</th>
                </tr>
            </thead>
            <tbody>
                <tr>
                    <td>Nomor Transaksi</td>
                    <td><strong>%s</strong></td>
                </tr>
                <tr>
                    <td>Nomor PO</td>
                    <td>%s</td>
                </tr>
                <tr>
                    <td>Tanggal Pemesanan</td>
                    <td>%s</td>
                </tr>
                <tr>
                    <td>Tanggal Jatuh Tempo</td>
                    <td><strong style="color: #b91c1c;">%s</strong></td>
                </tr>
                <tr>
                    <td>Total Nilai Pesanan</td>
                    <td>%s</td>
                </tr>
                <tr>
                    <td>Sisa Tagihan</td>
                    <td><strong style="color: #b91c1c;">%s</strong></td>
                </tr>
                <tr>
                    <td>Status Pembayaran</td>
                    <td><span class="badge badge-info">%s</span></td>
                </tr>
            </tbody>
        </table>
        <p>Mohon segera melakukan pelunasan pembayaran ke rekening resmi kami berikut:</p>
        <div style="background-color: #f8fafc; padding: 16px; border-radius: 8px; border: 1px solid #e2e8f0; margin: 16px 0;">
            <p style="margin: 0; font-weight: bold; color: #1e3a8a;">Bank Central Asia (BCA)</p>
            <p style="margin: 4px 0;">No. Rekening: <strong>123-456-7890</strong></p>
            <p style="margin: 0;">Atas Nama: <strong>PT WIRA TEKNIK</strong></p>
        </div>
        <p>Jika Anda sudah melakukan transfer pembayaran, abaikan email ini atau hubungi tim administrasi keuangan kami untuk konfirmasi rekonsiliasi.</p>
        <p>Terima kasih atas perhatian dan kerja sama Anda.</p>
        <p>Salam hangat,<br>Tim Wira Teknik</p>`,
		order.RecipientName,
		order.TransactionNo,
		order.PoNo,
		order.TransactionNo,
		order.PoNo,
		formatDateIndo(orderDate),
		formatDateIndo(dueDate),
		formatIDR(order.TotalAmountToPay),
		formatIDR(order.RemainingBalance),
		strings.Title(string(order.PaymentStatus)),
	)

	htmlContent := getEmailHeader("Peringatan Jatuh Tempo Pembayaran") + body + getEmailFooter()
	subject := fmt.Sprintf("Pemberitahuan Jatuh Tempo Pembayaran - %s", order.TransactionNo)

	return SendEmail(order.RecipientEmail, subject, htmlContent)
}

// Send3MonthOverduePaymentReminders scans for orders created 3 months ago (or older)
// that are unpaid or partially paid, and sends them a due reminder email.
func Send3MonthOverduePaymentReminders() (int, error) {
	// 3 months ago limit
	threeMonthsAgo := time.Now().AddDate(0, -3, 0)

	var orders []models.Order
	// Find all orders older than 3 months
	err := config.DB.
		Preload("Items").
		Preload("Shipments").
		Preload("Shipments.Invoice").
		Where("order_date <= ?", threeMonthsAgo).
		Find(&orders).Error
	if err != nil {
		return 0, err
	}

	sentCount := 0
	for _, order := range orders {
		// Calculate balance details
		computeOrderPaymentInfo(&order)

		// Send email if there's remaining balance
		if order.PaymentStatus != models.PaymentStatusPaid && order.RemainingBalance > 0 {
			if order.RecipientEmail != "" {
				err := SendPaymentDueReminderEmail(order.ID)
				if err == nil {
					sentCount++
				} else {
					fmt.Printf("Error sending due reminder for order %s: %v\n", order.ID, err)
				}
			}
		}
	}

	return sentCount, nil
}
