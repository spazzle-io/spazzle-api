package db

import (
	"math/big"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

func TestParseWeiStrToBigInt(t *testing.T) {
	testCases := []struct {
		name        string
		weiStr      string
		checkResult func(res *big.Int, err error)
	}{
		{
			name:   "success",
			weiStr: "1",
			checkResult: func(res *big.Int, err error) {
				require.NoError(t, err)
				require.Equal(t, big.NewInt(1), res)
			},
		},
		{
			name:   "success - large wei value",
			weiStr: "100000000000000000000",
			checkResult: func(res *big.Int, err error) {
				require.NoError(t, err)
				require.Equal(t, "100000000000000000000", res.String())
			},
		},
		{
			name:   "empty wei string",
			weiStr: "",
			checkResult: func(res *big.Int, err error) {
				require.Nil(t, res)
				require.Error(t, err)
			},
		},
		{
			name:   "invalid wei string",
			weiStr: "abc",
			checkResult: func(res *big.Int, err error) {
				require.Nil(t, res)
				require.Error(t, err)
			},
		},
		{
			name:   "negative wei string",
			weiStr: "-100",
			checkResult: func(res *big.Int, err error) {
				require.Nil(t, res)
				require.Error(t, err)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := ParseWeiStrToBigInt(tc.weiStr)
			tc.checkResult(res, err)
		})
	}
}

func TestParseDBNumericWeiToStr(t *testing.T) {
	testCases := []struct {
		name        string
		numeric     pgtype.Numeric
		checkResult func(res string, err error)
	}{
		{
			name: "success",
			numeric: pgtype.Numeric{
				Int:   big.NewInt(12),
				Exp:   17,
				Valid: true,
			},
			checkResult: func(res string, err error) {
				require.NoError(t, err)
				require.Equal(t, "1200000000000000000", res)
			},
		},
		{
			name: "invalid numeric",
			numeric: pgtype.Numeric{
				Int:   big.NewInt(12),
				Exp:   17,
				Valid: false,
			},
			checkResult: func(res string, err error) {
				require.Error(t, err)
				require.Empty(t, res)
			},
		},
		{
			name: "NaN numeric",
			numeric: pgtype.Numeric{
				Int:   big.NewInt(12),
				Exp:   17,
				Valid: true,
				NaN:   true,
			},
			checkResult: func(res string, err error) {
				require.Error(t, err)
				require.Empty(t, res)
			},
		},
		{
			name: "non-finite numeric",
			numeric: pgtype.Numeric{
				Valid:            true,
				InfinityModifier: pgtype.Infinity,
			},
			checkResult: func(res string, err error) {
				require.Error(t, err)
				require.Empty(t, res)
			},
		},
		{
			name: "nil numeric",
			numeric: pgtype.Numeric{
				Int:   nil,
				Valid: true,
			},
			checkResult: func(res string, err error) {
				require.NoError(t, err)
				require.Equal(t, "0", res)
			},
		},
		{
			name: "negative numeric",
			numeric: pgtype.Numeric{
				Int:   big.NewInt(-12),
				Exp:   17,
				Valid: true,
			},
			checkResult: func(res string, err error) {
				require.Error(t, err)
				require.Empty(t, res)
			},
		},
		{
			name: "zero exponent numeric",
			numeric: pgtype.Numeric{
				Int:   big.NewInt(12),
				Exp:   0,
				Valid: true,
			},
			checkResult: func(res string, err error) {
				require.NoError(t, err)
				require.Equal(t, "12", res)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := ParseDBNumericWeiToStr(tc.numeric)
			tc.checkResult(res, err)
		})
	}
}
