package db

import (
	"errors"
	"math/big"

	commonUtil "github.com/spazzle-io/spazzle-api/libs/common/util"

	"github.com/jackc/pgx/v5/pgtype"
)

func ParseDBNumericToWei(numeric pgtype.Numeric) (commonUtil.Wei, error) {
	if !numeric.Valid {
		return commonUtil.Wei{}, errors.New("invalid DB numeric")
	}

	if numeric.NaN {
		return commonUtil.Wei{}, errors.New("DB numeric is NaN")
	}

	if numeric.InfinityModifier != pgtype.Finite {
		return commonUtil.Wei{}, errors.New("DB numeric is infinite")
	}

	if numeric.Int == nil {
		return commonUtil.ZeroWei(), nil
	}

	if numeric.Exp < 0 {
		return commonUtil.Wei{}, errors.New("DB numeric has fractional value")
	}

	result := numeric.Int
	if numeric.Exp > 0 {
		pow := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(numeric.Exp)), nil)
		result = new(big.Int).Mul(numeric.Int, pow)
	}

	return commonUtil.NewWeiFromBigInt(result)
}
