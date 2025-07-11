package util

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
)

const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func GenerateRandomAlphanumericString(length int) (string, error) {
	result := make([]byte, length)
	for i := range result {
		randomIndex, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		result[i] = charset[randomIndex.Int64()]
	}

	return string(result), nil
}

func GenerateRandomNumericString(length int) (string, error) {
	maxNum := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(length)), nil)

	randomNum, err := rand.Int(rand.Reader, maxNum)
	if err != nil {
		return "", fmt.Errorf("could not generate random number: %w", err)
	}

	randomStr := fmt.Sprintf("%0*s", length, randomNum)

	return randomStr, nil
}

func NormalizeHexString(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "0x") {
		return "0x" + s
	}
	return s
}
