package utils

import "regexp"

// package-level compiled regexes — compiled once at startup, not on every call.
var (
	reEmail    = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	reLetter   = regexp.MustCompile(`[a-zA-Z]`)
	reDigit    = regexp.MustCompile(`[0-9]`)
	reUsername = regexp.MustCompile(`^[a-z0-9_.]+$`)
)

// IsValidEmail checks whether a string is a valid email format.
func IsValidEmail(email string) bool {
	return reEmail.MatchString(email)
}

// IsStrongPassword checks minimum 8 chars, at least one letter and one digit.
func IsStrongPassword(password string) bool {
	if len(password) < 8 {
		return false
	}
	return reLetter.MatchString(password) && reDigit.MatchString(password)
}

// IsValidUsername checks that a username contains only lowercase letters,
// digits, underscores, or dots. No spaces allowed. Length 3-30.
func IsValidUsername(name string) bool {
	if len(name) < 3 || len(name) > 30 {
		return false
	}
	return reUsername.MatchString(name)
}
