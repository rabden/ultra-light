package core

import "testing"

func TestEval(t *testing.T) {
	for q, want := range map[string]float64{
		"1 + 2 * 3": 7, "(1+2)*3": 9, "2^3^2": 512, "sqrt(9)": 3, "3 × 4": 12,
		"10 ÷ 4": 2.5, "-2*2": -4, "2e3 + 0": 2000, "7 % 3 =": 1, "1_000 * 2": 2000,
		"2 − 1": 1, "abs(-3)+0": 3,
	} {
		v, ok := Eval(q)
		if !ok || v != want {
			t.Errorf("%q = %v %v, want %v", q, v, ok, want)
		}
	}
	for _, q := range []string{"42", "safari", "e", "1/0", "2 +", "sqrt(-1)", "SQRT(4)", "", "=", "1__0+1"} {
		if v, ok := Eval(q); ok {
			t.Errorf("%q = %v, want no answer", q, v)
		}
	}
}

func TestFormat(t *testing.T) {
	for _, c := range []struct {
		v       float64
		grouped bool
		want    string
	}{
		{1048576, true, "1,048,576"}, {1048576, false, "1048576"}, {-1000, true, "-1,000"},
		{2.5, true, "2.5"}, {1.0 / 3.0, true, "0.3333333333"}, {0, true, "0"}, {999, true, "999"},
		{1e20, true, "100000000000000000000"},
	} {
		if got := FormatNumber(c.v, c.grouped); got != c.want {
			t.Errorf("format(%v, %v) = %q, want %q", c.v, c.grouped, got, c.want)
		}
	}
}
