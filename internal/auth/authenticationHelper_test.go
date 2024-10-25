package auth

import (
	"testing"
)

func TestHashPassword(t *testing.T) {
	password := "password"
	hash, err := HashPassword(password)
	if err != nil {
		t.Error(err)
	}
	if hash == password {
		t.Fatalf("\nHash: %v\nPassword: %v", hash, password)
	} else {
		t.Logf("\nHash: %v\nPassword: %v", hash, password)
	}
}

func TestComparePassword(t *testing.T) {
	password := "password"
	hash, err := HashPassword(password)
	if err != nil {
		t.Error(err)
	}
	err = CheckPasswordHash(password, hash)
	if err != nil {
		t.Error(err)
		return
	} else {
		t.Logf("\nHash: %v\nPassword: %v", hash, password)
	}
}
