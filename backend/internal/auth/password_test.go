package auth

import (
	"strings"
	"testing"
)

func TestPasswordHashAndVerify(t *testing.T) {
	password := strings.Repeat("long-password-", 20)
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPassword(hash, password) {
		t.Fatal("correct password was rejected")
	}
	if VerifyPassword(hash, password+"wrong") {
		t.Fatal("incorrect password was accepted")
	}
}
