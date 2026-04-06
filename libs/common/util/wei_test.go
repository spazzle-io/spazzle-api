package util

import (
	"math/big"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewWei(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "valid integer", input: "1000000000000000000", want: "1000000000000000000"},
		{name: "zero", input: "0", want: "0"},
		{name: "negative", input: "-500", want: "-500"},
		{name: "large valid 78 digits", input: strings.Repeat("9", 78), want: strings.Repeat("9", 78)},
		{name: "too long 79 digits", input: strings.Repeat("9", 79), wantErr: true},
		{name: "empty string", input: "", wantErr: true},
		{name: "non-numeric", input: "abc", wantErr: true},
		{name: "decimal", input: "1.5", wantErr: true},
		{name: "hex", input: "0xff", wantErr: true},
		{name: "whitespace", input: " 123 ", wantErr: true},
		{name: "leading plus", input: "+123", want: "123"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w, err := NewWei(tc.input)
			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.want, w.String())
		})
	}
}

func TestNewWeiFromBigInt(t *testing.T) {
	tests := []struct {
		name    string
		input   *big.Int
		want    string
		wantErr bool
	}{
		{name: "valid", input: big.NewInt(1000), want: "1000"},
		{name: "zero", input: big.NewInt(0), want: "0"},
		{name: "negative", input: big.NewInt(-500), want: "-500"},
		{name: "nil", input: nil, wantErr: true},
		{
			name: "exceeds max magnitude",
			input: func() *big.Int {
				b, _ := new(big.Int).SetString(strings.Repeat("9", 79), 10)
				return b
			}(),
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w, err := NewWeiFromBigInt(tc.input)
			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.want, w.String())
		})
	}
}

func TestNewWeiFromBigInt_DefensiveCopy(t *testing.T) {
	original := big.NewInt(100)
	w, err := NewWeiFromBigInt(original)
	require.NoError(t, err)

	original.SetInt64(999)
	require.Equal(t, "100", w.String())
}

func TestNewNonNegativeWei(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "positive", input: "100", want: "100"},
		{name: "zero", input: "0", want: "0"},
		{name: "negative", input: "-1", wantErr: true},
		{name: "invalid", input: "abc", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w, err := NewNonNegativeWei(tc.input)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, w.String())
		})
	}
}

func TestMustNewWei(t *testing.T) {
	t.Run("valid does not panic", func(t *testing.T) {
		require.NotPanics(t, func() {
			w := MustNewWei("123")
			require.Equal(t, "123", w.String())
		})
	})

	t.Run("invalid panics", func(t *testing.T) {
		require.Panics(t, func() {
			MustNewWei("abc")
		})
	})
}

func TestZeroWei(t *testing.T) {
	w := ZeroWei()
	require.Equal(t, "0", w.String())
}

func TestWeiAdd(t *testing.T) {
	tests := []struct {
		name string
		a    string
		b    string
		want string
	}{
		{name: "positive + positive", a: "100", b: "200", want: "300"},
		{name: "positive + negative", a: "100", b: "-30", want: "70"},
		{name: "zero + zero", a: "0", b: "0", want: "0"},
		{name: "large values", a: strings.Repeat("9", 77), b: "1", want: "1" + strings.Repeat("0", 77)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := MustNewWei(tc.a)
			b := MustNewWei(tc.b)
			require.Equal(t, tc.want, a.Add(b).String())
		})
	}
}

func TestWeiSub(t *testing.T) {
	tests := []struct {
		name string
		a    string
		b    string
		want string
	}{
		{name: "positive result", a: "300", b: "100", want: "200"},
		{name: "negative result", a: "100", b: "300", want: "-200"},
		{name: "zero result", a: "100", b: "100", want: "0"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := MustNewWei(tc.a)
			b := MustNewWei(tc.b)
			require.Equal(t, tc.want, a.Sub(b).String())
		})
	}
}

func TestWeiMul(t *testing.T) {
	tests := []struct {
		name string
		a    string
		n    int64
		want string
	}{
		{name: "basic", a: "100", n: 5, want: "500"},
		{name: "by zero", a: "100", n: 0, want: "0"},
		{name: "by negative", a: "100", n: -3, want: "-300"},
		{name: "negative by negative", a: "-100", n: -3, want: "300"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := MustNewWei(tc.a)
			require.Equal(t, tc.want, a.Mul(tc.n).String())
		})
	}
}

func TestWeiDiv(t *testing.T) {
	tests := []struct {
		name    string
		a       string
		divisor int64
		want    string
		wantErr bool
	}{
		{name: "even division", a: "1000", divisor: 10, want: "100"},
		{name: "truncated division", a: "1000", divisor: 3, want: "333"},
		{name: "divisor larger than value", a: "5", divisor: 10, want: "0"},
		{name: "divide by zero", a: "100", divisor: 0, wantErr: true},
		{name: "negative divisor", a: "100", divisor: -2, want: "-50"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := MustNewWei(tc.a)
			result, err := a.Div(tc.divisor)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, result.String())
		})
	}
}

func TestWeiBigInt_DefensiveCopy(t *testing.T) {
	w := MustNewWei("100")
	b := w.BigInt()

	b.SetInt64(999)
	require.Equal(t, "100", w.String())
}

func TestWeiString_UninitializedPanics(t *testing.T) {
	require.Panics(t, func() {
		w := Wei{}
		_ = w.String()
	})
}

func TestWeiBigInt_UninitializedPanics(t *testing.T) {
	require.Panics(t, func() {
		w := Wei{}
		w.BigInt()
	})
}
