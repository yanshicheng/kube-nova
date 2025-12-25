package utils

import (
	"github.com/google/uuid"
)

// GenerateRandomID 使用 UUID 库生成一个随机的 JWT ID
func GenerateRandomID() (string, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return "", err
	}
	return id.String(), nil
}
