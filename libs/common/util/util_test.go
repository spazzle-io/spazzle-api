package util

import (
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
