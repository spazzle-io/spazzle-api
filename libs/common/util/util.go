package util

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strings"

	"github.com/rivo/uniseg"
	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
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

// Uint64ToUint32 safely converts uint64 to uint32 with bounds checking.
// Returns an error if the value cannot fit in uint32.
func Uint64ToUint32(n uint64) (uint32, error) {
	if n > math.MaxUint32 {
		return 0, fmt.Errorf("uint64 value %d out of uint32 range", n)
	}
	return uint32(n), nil
}

// Uint64ToInt64 safely converts uint64 to int64 with bounds checking.
// Returns an error if the value exceeds math.MaxInt64.
func Uint64ToInt64(n uint64) (int64, error) {
	if n > math.MaxInt64 {
		return 0, fmt.Errorf("uint64 value %d overflows int64", n)
	}
	return int64(n), nil
}

// Uint32ToUint8 safely converts uint32 to uint8 with bounds checking.
// Returns an error if the value exceeds math.MaxUint8.
func Uint32ToUint8(n uint32) (uint8, error) {
	if n > math.MaxUint8 {
		return 0, fmt.Errorf("uint32 value %d out of uint8 range", n)
	}
	return uint8(n), nil
}

// Int32ToUint8 safely converts int32 to uint8 with bounds checking.
// Returns an error if the value is negative or exceeds math.MaxUint8.
func Int32ToUint8(n int32) (uint8, error) {
	if n < 0 || n > math.MaxUint8 {
		return 0, fmt.Errorf("int32 value %d out of uint8 range", n)
	}
	return uint8(n), nil
}

// IntToUint32 safely converts int to uint32 with bounds checking.
// Returns an error if the value is negative or exceeds math.MaxUint32.
func IntToUint32(n int) (uint32, error) {
	if n < 0 || uint64(n) > math.MaxUint32 {
		return 0, fmt.Errorf("int value %d out of uint32 range", n)
	}
	return uint32(n), nil
}

// IntToInt32 safely converts int to int32 with bounds checking.
// Returns an error if the value cannot fit in int32.
func IntToInt32(n int) (int32, error) {
	if n < math.MinInt32 || n > math.MaxInt32 {
		return 0, fmt.Errorf("int value %d out of int32 range", n)
	}
	return int32(n), nil
}

// RandomIndices returns k unique random indices in the range [0, n),
// sampled uniformly without replacement.
//
// This makes use of a partial Fisher–Yates shuffle to ensure each index
// is chosen with equal probability.
func RandomIndices(n int, k int) ([]int, error) {
	if k > n {
		return nil, errors.New("k must be less than or equal to n")
	}

	indices := make([]int, n)
	for i := range indices {
		indices[i] = i
	}

	for i := 0; i < k; i++ {
		randomOffset, err := rand.Int(rand.Reader, big.NewInt(int64(n-i)))
		if err != nil {
			return nil, err
		}
		swapIndex := i + int(randomOffset.Int64())
		indices[i], indices[swapIndex] = indices[swapIndex], indices[i]
	}

	return indices[:k], nil
}

// GraphemeLen returns the number of visible Unicode characters in a string.
func GraphemeLen(str string) int {
	count := 0
	graphemes := uniseg.NewGraphemes(str)

	for graphemes.Next() {
		count++
	}

	return count
}

// CharAt returns the Unicode character at the specified index in a string.
func CharAt(str string, index int) (string, error) {
	i := 0
	graphemes := uniseg.NewGraphemes(str)

	for graphemes.Next() {
		if i == index {
			return graphemes.Str(), nil
		}
		i++
	}

	return "", fmt.Errorf("index out of range")
}

// EqualText returns whether two strings are equal in a Unicode-correct, case-insensitive way
// i.e. the strings are visually identical without taking case into account.
func EqualText(a string, b string) bool {
	fold := cases.Fold()

	normalizedA := norm.NFC.String(fold.String(a))
	normalizedB := norm.NFC.String(fold.String(b))

	return normalizedA == normalizedB
}
