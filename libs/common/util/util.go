package util

import (
	"crypto/rand"
	"fmt"
	"math"
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

// Int64ToInt32 safely converts int64 to int32 with bounds checking.
// Returns an error if the value cannot fit in int32.
func Int64ToInt32(n int64) (int32, error) {
	if n < math.MinInt32 || n > math.MaxInt32 {
		return 0, fmt.Errorf("int64 value %d out of int32 range", n)
	}
	return int32(n), nil
}

// IntToInt32 safely converts int to int32 with bounds checking.
// Returns an error if the value cannot fit in int32.
func IntToInt32(n int) (int32, error) {
	if n < math.MinInt32 || n > math.MaxInt32 {
		return 0, fmt.Errorf("int value %d out of int32 range", n)
	}
	return int32(n), nil
}
