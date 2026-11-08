package auth

import (
	"errors"

	"golang.org/x/crypto/bcrypt"
)

const bcryptCost = 12

// HashPassword generates a bcrypt hash of the plain-text password.
func HashPassword(plain string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), bcryptCost)
	if err != nil {
		return "", errors.New("auth: failed to hash password")
	}
	return string(hash), nil
}

// CheckPassword returns nil if plain matches the stored hash,
// or ErrInvalidCredentials otherwise. The caller should never
// distinguish between "user not found" and "wrong password" in
// API responses — both map to 401.
func CheckPassword(hash, plain string) error {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain))
	if err != nil {
		return ErrInvalidCredentials
	}
	return nil
}

// ErrInvalidCredentials is returned when email/password do not match.
// Kept as a typed sentinel so handlers can respond with 401 without
// leaking whether the email exists.
var ErrInvalidCredentials = errors.New("auth: invalid credentials")
