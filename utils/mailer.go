package utils

import (
	"fmt"
	"net/smtp"
	"os"
)

// SendOTPEmail sends a password reset OTP to the given email address.
func SendOTPEmail(toEmail, otp string) error {
	smtpHost := os.Getenv("SMTP_HOST")
	smtpPort := os.Getenv("SMTP_PORT")
	smtpUser := os.Getenv("SMTP_USER")
	smtpPass := os.Getenv("SMTP_PASS")
	fromName := os.Getenv("APP_NAME")

	if fromName == "" {
		fromName = "Wira Teknik"
	}

	subject := "Kode Verifikasi Reset Password"
	body := fmt.Sprintf(`Halo,

Anda menerima email ini karena ada permintaan reset password untuk akun Anda di %s.

Kode Verifikasi Anda: %s

Kode ini berlaku selama 15 menit. Jangan bagikan kode ini kepada siapapun.

Jika Anda tidak meminta reset password, abaikan email ini.

Salam,
Tim %s`, fromName, otp, fromName)

	message := fmt.Sprintf(
		"From: %s <%s>\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		fromName, smtpUser, toEmail, subject, body,
	)

	auth := smtp.PlainAuth("", smtpUser, smtpPass, smtpHost)
	addr := fmt.Sprintf("%s:%s", smtpHost, smtpPort)

	return smtp.SendMail(addr, auth, smtpUser, []string{toEmail}, []byte(message))
}
