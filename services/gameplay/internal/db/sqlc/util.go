package db

import (
	"errors"
	"math/big"

	"github.com/jackc/pgx/v5/pgtype"
)

func ParseWeiStrToBigInt(s string) (*big.Int, error) {
	if s == "" {
		return nil, errors.New("empty wei string")
	}

	weiBigInt := new(big.Int)
	if _, ok := weiBigInt.SetString(s, 10); !ok {
		return nil, errors.New("invalid decimal wei")
	}

	if weiBigInt.Sign() < 0 {
		return nil, errors.New("wei must be non-negative")
	}

	return weiBigInt, nil
}

func ParseDBNumericWeiToStr(numeric pgtype.Numeric) (string, error) {
	if !numeric.Valid {
		return "", errors.New("invalid DB numeric")
	}

	if numeric.NaN {
		return "", errors.New("DB numeric is NaN")
	}

	if numeric.InfinityModifier != pgtype.Finite {
		return "", errors.New("DB numeric is infinite")
	}

	if numeric.Int == nil {
		return "0", nil
	}

	if numeric.Int.Sign() < 0 {
		return "", errors.New("DB numeric must be non-negative wei")
	}

	if numeric.Exp < 0 {
		return "", errors.New("DB numeric has fractional value, expected integer wei")
	}

	if numeric.Exp == 0 {
		return numeric.Int.String(), nil
	}

	pow := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(numeric.Exp)), nil)
	result := new(big.Int).Mul(numeric.Int, pow)

	return result.String(), nil
}
