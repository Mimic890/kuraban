package auth

import (
	"crypto/sha256"
	"encoding/hex"

	"golang.org/x/crypto/bcrypt"
)

// HashPassword pre-hashes to avoid bcrypt's 72-byte input limit.
func HashPassword(password string) (string, error) {
	sum := sha256.Sum256([]byte(password))
	hash, err := bcrypt.GenerateFromPassword([]byte(hex.EncodeToString(sum[:])), bcrypt.DefaultCost)
	return string(hash), err
}

func VerifyPassword(hash, password string) bool {
	sum := sha256.Sum256([]byte(password))
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(hex.EncodeToString(sum[:]))) == nil
}
