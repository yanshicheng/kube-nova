package utils

import (
	"encoding/base64"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"

	"golang.org/x/crypto/bcrypt"
)

func GeneratePassword() string {
	currentTime := time.Now()                       // 获取当前时间
	formattedDate := currentTime.Format("20060102") // 格式化日期为 YYYYMMDD
	fmt.Println("Ikubeops@" + formattedDate)
	fullPassword := "Ikubeops@" + formattedDate
	return strings.TrimSpace(fullPassword)
}

//	func GenerateIcon() string {
//		return fmt.Sprintf("%s/account/default.png", global.StaticDir)
//	}
func ValidatePassword(password string) bool {
	if len(password) < 8 {
		return false
	}

	var (
		hasUpper   = false
		hasLower   = false
		hasNumber  = false
		hasSpecial = false
	)

	for _, char := range password {
		switch {
		case unicode.IsUpper(char):
			hasUpper = true
		case unicode.IsLower(char):
			hasLower = true
		case unicode.IsDigit(char):
			hasNumber = true
		case unicode.IsPunct(char) || unicode.IsSymbol(char):
			hasSpecial = true
		}
	}

	return hasUpper && hasLower && hasNumber && hasSpecial
}

// DecodeBase64Password 解码使用Base64编码的密码
func DecodeBase64Password(encodedPassword string) (string, error) {
	decodedBytes, err := base64.StdEncoding.DecodeString(encodedPassword)
	if err != nil {
		return "", err // 如果解码失败，返回错误
	}
	// 去掉左右两边的空格和换行符
	return strings.TrimSpace(string(decodedBytes)), nil // 将解码后的字节转换为字符串并返回
}

func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func CheckPasswordComplexity(password string) bool {
	if len(password) < 12 {
		return false
	}

	hasDigit := regexp.MustCompile(`[0-9]`).MatchString
	hasUpper := regexp.MustCompile(`[A-Z]`).MatchString
	hasLower := regexp.MustCompile(`[a-z]`).MatchString
	hasSpecial := regexp.MustCompile(`[^a-zA-Z0-9]`).MatchString

	return hasDigit(password) && hasUpper(password) && hasLower(password) && hasSpecial(password)
}

// 密码加密
func EncryptPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}
