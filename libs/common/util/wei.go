package util

import (
	"errors"
	"fmt"
	"math/big"
)

const maxWeiStrLen = 78

type Wei struct {
	val *big.Int
}

func NewWei(s string) (Wei, error) {
	if len(s) > maxWeiStrLen {
		return Wei{}, errors.New("wei string too long")
	}

	b, ok := new(big.Int).SetString(s, 10)
	if !ok {
		return Wei{}, fmt.Errorf("invalid wei string: %q", s)
	}

	return Wei{
		val: b,
	}, nil
}

func NewWeiFromBigInt(b *big.Int) (Wei, error) {
	if b == nil {
		return Wei{}, errors.New("nil big.Int")
	}

	if len(b.Text(10)) > maxWeiStrLen {
		return Wei{}, errors.New("big.Int exceeds max wei magnitude")
	}

	return Wei{
		val: new(big.Int).Set(b),
	}, nil
}

func NewNonNegativeWei(s string) (Wei, error) {
	w, err := NewWei(s)
	if err != nil {
		return Wei{}, err
	}

	if err := w.RequireNonNegative(); err != nil {
		return Wei{}, err
	}

	return w, nil
}

func MustNewWei(s string) Wei {
	w, err := NewWei(s)
	if err != nil {
		panic(fmt.Sprintf("MustNewWei(%q): %v", s, err))
	}

	return w
}

func (w Wei) RequireNonNegative() error {
	if w.val.Sign() < 0 {
		return errors.New("wei must be non-negative")
	}

	return nil
}

func ZeroWei() Wei {
	return Wei{
		val: new(big.Int),
	}
}

func (w Wei) String() string {
	if w.val == nil {
		panic("uninitialized Wei")
	}

	return w.val.String()
}

func (w Wei) BigInt() *big.Int {
	if w.val == nil {
		panic("uninitialized Wei")
	}

	return new(big.Int).Set(w.val)
}

func (w Wei) Add(other Wei) Wei {
	return Wei{
		val: new(big.Int).Add(w.val, other.val),
	}
}

func (w Wei) Sub(other Wei) Wei {
	return Wei{
		val: new(big.Int).Sub(w.val, other.val),
	}
}

func (w Wei) Mul(n int64) Wei {
	return Wei{
		val: new(big.Int).Mul(w.val, big.NewInt(n)),
	}
}

func (w Wei) Div(divisor int64) (Wei, error) {
	if divisor == 0 {
		return Wei{}, errors.New("division by zero")
	}

	return Wei{
		val: new(big.Int).Div(w.val, big.NewInt(divisor)),
	}, nil
}
