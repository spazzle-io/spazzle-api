package util

import (
	"fmt"
	"math"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerateRandomAlphanumericString(t *testing.T) {
	expectedLen := 12
	generatedString, err := GenerateRandomAlphanumericString(expectedLen)

	require.NoError(t, err)
	require.NotEmpty(t, generatedString)
	require.Len(t, generatedString, expectedLen)
}

func TestGenerateRandomNumericString(t *testing.T) {
	expectedLen := 9
	randomNum, err := GenerateRandomNumericString(expectedLen)
	println(randomNum)

	require.NoError(t, err)
	require.NotEmpty(t, randomNum)
	require.Len(t, randomNum, expectedLen)
}

func TestNormalizeHexString(t *testing.T) {
	testCases := []struct {
		name        string
		inputHex    string
		expectedHex string
	}{
		{
			name:        "already normalized hex",
			inputHex:    "0x123",
			expectedHex: "0x123",
		},
		{
			name:        "with whitespace",
			inputHex:    " 0x123 ",
			expectedHex: "0x123",
		},
		{
			name:        "no prefix",
			inputHex:    "123",
			expectedHex: "0x123",
		},
		{
			name:        "no prefix with whitespace",
			inputHex:    "123 ",
			expectedHex: "0x123",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			normalizedHex := NormalizeHexString(tc.inputHex)
			require.Equal(t, tc.expectedHex, normalizedHex)
		})
	}
}

func TestInt64ToInt32(t *testing.T) {
	testCases := []struct {
		name     string
		input    int64
		expected int32
		success  bool
	}{
		{
			name:     "success",
			input:    1,
			expected: 1,
			success:  true,
		},
		{
			name:    "input too small",
			input:   int64(math.MinInt64),
			success: false,
		},
		{
			name:    "input too large",
			input:   int64(math.MaxInt64),
			success: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := Int64ToInt32(tc.input)
			if tc.success {
				require.NoError(t, err)
				require.Equal(t, tc.expected, result)
				return
			}

			require.Error(t, err)
		})
	}
}

func TestIntToInt32(t *testing.T) {
	testCases := []struct {
		name     string
		input    int
		expected int32
		success  bool
	}{
		{
			name:     "success",
			input:    1,
			expected: 1,
			success:  true,
		},
		{
			name:    "input too small",
			input:   math.MinInt,
			success: false,
		},
		{
			name:    "input too large",
			input:   math.MaxInt,
			success: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := IntToInt32(tc.input)
			if tc.success {
				require.NoError(t, err)
				require.Equal(t, tc.expected, result)
				return
			}

			require.Error(t, err)
		})
	}
}

func TestRandomIndices(t *testing.T) {
	testCases := []struct {
		name      string
		n         int
		k         int
		shouldErr bool
	}{
		{
			name:      "k < n",
			n:         5,
			k:         3,
			shouldErr: false,
		},
		{
			name:      "k = n",
			n:         5,
			k:         5,
			shouldErr: false,
		},
		{
			name:      "k > n",
			n:         5,
			k:         6,
			shouldErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			randomIndices, err := RandomIndices(tc.n, tc.k)
			if tc.shouldErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotEmpty(t, randomIndices)

			for _, i := range randomIndices {
				require.Less(t, i, tc.n)
				require.GreaterOrEqual(t, i, 0)
			}
		})
	}
}

func TestParseBigIntOrZero(t *testing.T) {
	testCases := []struct {
		name           string
		s              string
		expectedResult func() *big.Int
	}{
		{
			name: "success",
			s:    "1000000000000000000000",
			expectedResult: func() *big.Int {
				b, success := big.NewInt(0).SetUint64(0).SetString("1000000000000000000000", 10)
				require.True(t, success)
				return b
			},
		},
		{
			name: "large integer",
			s:    "12345678901234567890123456789012345678901234567890",
			expectedResult: func() *big.Int {
				b, success := big.NewInt(0).SetUint64(0).SetString("12345678901234567890123456789012345678901234567890", 10)
				require.True(t, success)
				return b
			},
		},
		{
			name: "invalid input",
			s:    "abc",
			expectedResult: func() *big.Int {
				return big.NewInt(0)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expectedResult(), ParseBigIntOrZero(tc.s))
		})
	}
}

func TestBigIntString(t *testing.T) {
	testCases := []struct {
		name           string
		bigInt         func() *big.Int
		expectedResult string
	}{
		{
			name: "success",
			bigInt: func() *big.Int {
				b, success := big.NewInt(0).SetUint64(0).SetString("1000000000000000000000", 10)
				require.True(t, success)
				return b
			},
			expectedResult: "1000000000000000000000",
		},
		{
			name: "large integer",
			bigInt: func() *big.Int {
				b, success := big.NewInt(0).SetUint64(0).SetString("12345678901234567890123456789012345678901234567890", 10)
				require.True(t, success)
				return b
			},
			expectedResult: "12345678901234567890123456789012345678901234567890",
		},
		{
			name: "nil input",
			bigInt: func() *big.Int {
				return nil
			},
			expectedResult: "0",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			b := tc.bigInt()
			require.Equal(t, tc.expectedResult, BigIntString(b))
		})
	}
}

func TestAddBigIntStrings(t *testing.T) {
	testCases := []struct {
		name           string
		a              string
		b              string
		expectedResult string
	}{
		{
			name:           "success",
			a:              "1000000000000000000000",
			b:              "1000000000000000000000",
			expectedResult: "2000000000000000000000",
		},
		{
			name:           "parse error - one string",
			a:              "1000000000000000000000",
			b:              "abc",
			expectedResult: "1000000000000000000000",
		},
		{
			name:           "parse error - both strings",
			a:              "invalid",
			b:              "abc",
			expectedResult: "0",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := AddBigIntStrings(tc.a, tc.b)
			require.Equal(t, tc.expectedResult, result)
		})
	}
}

func TestDivBigInstString(t *testing.T) {
	testCases := []struct {
		name           string
		s              string
		divisor        int64
		expectedResult string
	}{
		{
			name:           "success",
			s:              "1000000000000000000000",
			divisor:        100,
			expectedResult: "10000000000000000000",
		},
		{
			name:           "truncated division",
			s:              "1000000000000000000000",
			divisor:        9,
			expectedResult: "111111111111111111111",
		},
		{
			name:           "divisor larger than input s",
			s:              "5",
			divisor:        10,
			expectedResult: "0",
		},
		{
			name:           "negative divisor",
			s:              "10",
			divisor:        -1,
			expectedResult: "0",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := DivBigIntString(tc.s, tc.divisor)
			require.Equal(t, tc.expectedResult, result)
		})
	}
}

func TestGraphemeLen(t *testing.T) {
	testCases := []struct {
		str         string
		expectedLen int
	}{
		{
			str:         "hello world",
			expectedLen: 11,
		},
		{
			str:         "abc  ",
			expectedLen: 5,
		},
		{
			str:         "café",
			expectedLen: 4,
		},
		{
			str:         "🙊🤘🏿🚀",
			expectedLen: 3,
		},
		{
			str:         "猫狗",
			expectedLen: 2,
		},
		{
			str:         " 🤘🏿 ",
			expectedLen: 3,
		},
		{
			str:         " 猫狗",
			expectedLen: 3,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.str, func(t *testing.T) {
			length := GraphemeLen(tc.str)
			require.Equal(t, tc.expectedLen, length)
		})
	}
}

func TestCharAt(t *testing.T) {
	testCases := []struct {
		str          string
		index        int
		expectedChar string
		shouldErr    bool
	}{
		{
			str:          "abc",
			index:        1,
			expectedChar: "b",
			shouldErr:    false,
		},
		{
			str:          "café",
			index:        3,
			expectedChar: "é",
			shouldErr:    false,
		},
		{
			str:          "🙊🤘🏿🚀",
			index:        1,
			expectedChar: "🤘🏿",
			shouldErr:    false,
		},
		{
			str:          "猫狗",
			index:        1,
			expectedChar: "狗",
			shouldErr:    false,
		},
		{
			str:       "abc",
			index:     3,
			shouldErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.str, func(t *testing.T) {
			char, err := CharAt(tc.str, tc.index)
			if tc.shouldErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.expectedChar, char)
		})
	}
}

func TestEqualText(t *testing.T) {
	testCases := []struct {
		strA     string
		strB     string
		expected bool
	}{
		{
			strA:     "abc",
			strB:     "abc",
			expected: true,
		},
		{
			strA:     "abc",
			strB:     "aBc",
			expected: true,
		},
		{
			strA:     "abc",
			strB:     "Ab",
			expected: false,
		},
		{
			strA:     "é",
			strB:     "e\u0301",
			expected: true,
		},
		{
			strA:     "É",
			strB:     "e\u0301",
			expected: true,
		},
		{
			strA:     "ß",
			strB:     "SS",
			expected: true,
		},
		{
			strA:     "Σ",
			strB:     "ς",
			expected: true,
		},
		{
			strA:     "🤘🏿",
			strB:     "🤘🏿",
			expected: true,
		},
		{
			strA:     "🤘🏿",
			strB:     "🤘",
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%s/%s", tc.strA, tc.strB), func(t *testing.T) {
			require.Equal(t, tc.expected, EqualText(tc.strA, tc.strB))
		})
	}
}
