package utils

import "regexp"

// IsValidEmail checks whether a string is a valid email format.
func IsValidEmail(email string) bool {
	re := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	return re.MatchString(email)
}

// IsStrongPassword checks minimum 8 chars, at least one letter and one digit.
func IsStrongPassword(password string) bool {
	if len(password) < 8 {
		return false
	}
	hasLetter := regexp.MustCompile(`[a-zA-Z]`).MatchString(password)
	hasDigit := regexp.MustCompile(`[0-9]`).MatchString(password)
	return hasLetter && hasDigit
}

// IsValidUsername checks that a username contains only lowercase letters,
// digits, underscores, or dots. No spaces allowed. Length 3-30.
func IsValidUsername(name string) bool {
	if len(name) < 3 || len(name) > 30 {
		return false
	}
	re := regexp.MustCompile(`^[a-z0-9_.]+$`)
	return re.MatchString(name)
}
