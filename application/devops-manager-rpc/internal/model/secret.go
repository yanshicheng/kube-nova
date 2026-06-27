package model

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"io"
	"os"
	"strings"
)

const encryptedSecretPrefix = "enc:v1:"

func encryptSecret(plain string) (string, error) {
	if strings.TrimSpace(plain) == "" || strings.HasPrefix(plain, encryptedSecretPrefix) {
		return plain, nil
	}
	block, err := aes.NewCipher(secretKey())
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	payload := append(nonce, gcm.Seal(nil, nonce, []byte(plain), nil)...)
	return encryptedSecretPrefix + base64.StdEncoding.EncodeToString(payload), nil
}

func decryptSecret(value string) (string, error) {
	if strings.TrimSpace(value) == "" || !strings.HasPrefix(value, encryptedSecretPrefix) {
		return value, nil
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(value, encryptedSecretPrefix))
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(secretKey())
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", io.ErrUnexpectedEOF
	}
	nonce := raw[:gcm.NonceSize()]
	cipherText := raw[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, cipherText, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func secretKey() []byte {
	seed := os.Getenv("DEVOPS_SECRET_KEY")
	if strings.TrimSpace(seed) == "" {
		seed = "kube-nova-devops-default-secret"
	}
	sum := sha256.Sum256([]byte(seed))
	return sum[:]
}
