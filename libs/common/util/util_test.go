package util

import (
	"math"
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
