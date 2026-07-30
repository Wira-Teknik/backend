package utils

import (
	"crypto/rand"
	"math/big"
)

const otpLength = 6
const otpChars = "0123456789"

// GenerateOTP menghasilkan kode OTP numerik yang aman secara kriptografi dengan panjang tetap.
func GenerateOTP() (string, error) {
	otp := make([]byte, otpLength)
	for i := range otp {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(otpChars))))
		if err != nil {
			return "", err
		}
		otp[i] = otpChars[num.Int64()]
	}
	return string(otp), nil
}
