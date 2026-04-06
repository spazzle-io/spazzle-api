package db

import (
	"math/big"
	"strings"
	"testing"

	commonUtil "github.com/spazzle-io/spazzle-api/libs/common/util"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

func TestParseDBNumericToWei(t *testing.T) {
	testCases := []struct {
		name        string
		numeric     pgtype.Numeric
		checkResult func(res commonUtil.Wei, err error)
	}{
		{
			name: "success",
			numeric: pgtype.Numeric{
				Int:   big.NewInt(12),
				Exp:   17,
				Valid: true,
			},
			checkResult: func(res commonUtil.Wei, err error) {
				require.NoError(t, err)
				require.Equal(t, "1200000000000000000", res.String())
			},
		},
		{
			name: "invalid numeric",
			numeric: pgtype.Numeric{
				Int:   big.NewInt(12),
				Exp:   17,
				Valid: false,
			},
			checkResult: func(res commonUtil.Wei, err error) {
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
			checkResult: func(res commonUtil.Wei, err error) {
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
			checkResult: func(res commonUtil.Wei, err error) {
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
			checkResult: func(res commonUtil.Wei, err error) {
				require.NoError(t, err)
				require.Equal(t, "0", res.String())
			},
		},
		{
			name: "negative numeric",
			numeric: pgtype.Numeric{
				Int:   big.NewInt(-12),
				Exp:   17,
				Valid: true,
			},
			checkResult: func(res commonUtil.Wei, err error) {
				require.NoError(t, err)
				require.Equal(t, "-1200000000000000000", res.String())
			},
		},
		{
			name: "zero exponent numeric",
			numeric: pgtype.Numeric{
				Int:   big.NewInt(12),
				Exp:   0,
				Valid: true,
			},
			checkResult: func(res commonUtil.Wei, err error) {
				require.NoError(t, err)
				require.Equal(t, "12", res.String())
			},
		},
		{
			name: "fractional numeric",
			numeric: pgtype.Numeric{
				Int:   big.NewInt(123),
				Exp:   -2,
				Valid: true,
			},
			checkResult: func(res commonUtil.Wei, err error) {
				require.Error(t, err)
				require.Empty(t, res)
			},
		},
		{
			name: "exceeds max wei magnitude",
			numeric: pgtype.Numeric{
				Int:   big.NewInt(1),
				Exp:   79,
				Valid: true,
			},
			checkResult: func(res commonUtil.Wei, err error) {
				require.Error(t, err)
				require.Empty(t, res)
			},
		},
		{
			name: "zero value explicit",
			numeric: pgtype.Numeric{
				Int:   big.NewInt(0),
				Exp:   0,
				Valid: true,
			},
			checkResult: func(res commonUtil.Wei, err error) {
				require.NoError(t, err)
				require.Equal(t, "0", res.String())
			},
		},
		{
			name: "large valid numeric",
			numeric: pgtype.Numeric{
				Int:   big.NewInt(1),
				Exp:   77,
				Valid: true,
			},
			checkResult: func(res commonUtil.Wei, err error) {
				require.NoError(t, err)
				require.Equal(t, "1"+strings.Repeat("0", 77), res.String())
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := ParseDBNumericToWei(tc.numeric)
			tc.checkResult(res, err)
		})
	}
}
