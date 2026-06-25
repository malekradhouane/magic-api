package encrypt

import "golang.org/x/crypto/bcrypt"

// PasswordHashCost is the bcrypt cost used for all password hashes.
// bcrypt.DefaultCost (10) is too weak for 2026 hardware. Cost 12 yields
// ~250 ms on modern servers, which is the recommended baseline by OWASP.
// Increase this number when CPUs get faster.
const PasswordHashCost = 12

// Hash returns the bcrypt hash of the given password using PasswordHashCost.
func Hash(password string) ([]byte, error) {
	return bcrypt.GenerateFromPassword([]byte(password), PasswordHashCost)
}

// VerifyPassword compares a bcrypt hash with a clear-text password.
// It returns bcrypt.ErrMismatchedHashAndPassword on mismatch.
func VerifyPassword(hashedPassword, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
}
