package utils

import (
	"crypto/rand"
	"encoding/hex"
)

func GenerateSecureToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func GenerateToken() (string, error) {
	return GenerateSecureToken(64)
}
