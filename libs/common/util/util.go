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

// ParseBigIntOrZero parses a base-10 integer string and returns 0 if parsing fails.
func ParseBigIntOrZero(s string) *big.Int {
	b, ok := new(big.Int).SetString(s, 10)
	if !ok {
		return big.NewInt(0)
	}

	return b
}

// BigIntString returns the base-10 string representation of b, or "0" if b is nil.
func BigIntString(b *big.Int) string {
	if b == nil {
		return "0"
	}

	return b.String()
}

// AddBigIntStrings adds two base-10 integer strings and returns the sum as a string.
// If parsing of either strings fails, it is set to 0.
func AddBigIntStrings(a string, b string) string {
	parsedA := ParseBigIntOrZero(a)
	parsedB := ParseBigIntOrZero(b)

	return BigIntString(new(big.Int).Add(parsedA, parsedB))
}

// DivBigIntString divides a base-10 integer string by a divisor and returns the result as a string.
// Returns 0 if divisor <= 0.
func DivBigIntString(s string, divisor int64) string {
	if divisor <= 0 {
		return "0"
	}

	parsedS := ParseBigIntOrZero(s)
	return new(big.Int).Div(parsedS, big.NewInt(divisor)).String()
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
