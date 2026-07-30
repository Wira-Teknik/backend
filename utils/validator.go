package utils

import "regexp"

// regex yang dikompilasi di tingkat paket — dikompilasi sekali saat startup, bukan pada setiap panggilan.
var (
	reEmail    = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	reLetter   = regexp.MustCompile(`[a-zA-Z]`)
	reDigit    = regexp.MustCompile(`[0-9]`)
	reUsername = regexp.MustCompile(`^[a-z0-9_.]+$`)
)

// IsValidEmail memeriksa apakah format string email valid.
func IsValidEmail(email string) bool {
	return reEmail.MatchString(email)
}

// IsStrongPassword memeriksa kekuatan kata sandi, minimal 8 karakter, serta memiliki minimal satu huruf dan satu angka.
func IsStrongPassword(password string) bool {
	if len(password) < 8 {
		return false
	}
	return reLetter.MatchString(password) && reDigit.MatchString(password)
}

// IsValidUsername memeriksa apakah nama pengguna hanya berisi huruf kecil, angka, garis bawah, atau titik, tanpa spasi, dengan panjang 3-30 karakter.
func IsValidUsername(name string) bool {
	if len(name) < 3 || len(name) > 30 {
		return false
	}
	return reUsername.MatchString(name)
}
