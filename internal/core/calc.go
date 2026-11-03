package core

import (
	"errors"
	"math"
	"strconv"
	"strings"
)

// Math typed into the bar, answered as it's typed: + - * / % ^, × and ÷,
// parentheses, pi and e, and sqrt, sin, cos, tan, ln, log, abs. A query
// counts as math only with an operator or a function in it, so "42" or "e"
// alone stay searches. Port of calc.zig.

var (
	errSyntax = errors.New("syntax")
	errDomain = errors.New("domain")
)

type parser struct {
	s string
	i int
	// An operator or a function was seen: this is math, not a name.
	math bool
}

func (p *parser) skip() {
	for p.i < len(p.s) && p.s[p.i] == ' ' {
		p.i++
	}
}

func (p *parser) peek() byte {
	p.skip()
	if p.i < len(p.s) {
		return p.s[p.i]
	}
	return 0
}

// op takes ascii, or its UTF-8 twin (× ÷ −) when given.
func (p *parser) op(ascii byte, utf8 string) bool {
	p.skip()
	if p.i < len(p.s) && p.s[p.i] == ascii {
		p.i++
		return true
	}
	if utf8 != "" && strings.HasPrefix(p.s[p.i:], utf8) {
		p.i += len(utf8)
		return true
	}
	return false
}

func (p *parser) expr() (float64, error) {
	v, err := p.term()
	if err != nil {
		return 0, err
	}
	for {
		switch {
		case p.op('+', ""):
			p.math = true
			t, err := p.term()
			if err != nil {
				return 0, err
			}
			v += t
		case p.op('-', "−"):
			p.math = true
			t, err := p.term()
			if err != nil {
				return 0, err
			}
			v -= t
		default:
			return v, nil
		}
	}
}

func (p *parser) term() (float64, error) {
	v, err := p.power()
	if err != nil {
		return 0, err
	}
	for {
		switch {
		case p.op('*', "×"):
			p.math = true
			d, err := p.power()
			if err != nil {
				return 0, err
			}
			v *= d
		case p.op('/', "÷"):
			p.math = true
			d, err := p.power()
			if err != nil {
				return 0, err
			}
			if d == 0 {
				return 0, errDomain
			}
			v /= d
		case p.op('%', ""):
			p.math = true
			d, err := p.power()
			if err != nil {
				return 0, err
			}
			if d == 0 {
				return 0, errDomain
			}
			v = math.Mod(v, d)
		default:
			return v, nil
		}
	}
}

// power is right-associative: 2^3^2 is 2^9.
func (p *parser) power() (float64, error) {
	base, err := p.unary()
	if err != nil {
		return 0, err
	}
	if p.op('^', "") {
		p.math = true
		e, err := p.power()
		if err != nil {
			return 0, err
		}
		return math.Pow(base, e), nil
	}
	return base, nil
}

func (p *parser) unary() (float64, error) {
	if p.op('-', "−") {
		v, err := p.unary()
		return -v, err
	}
	if p.op('+', "") {
		return p.unary()
	}
	return p.primary()
}

func isDigit(ch byte) bool { return ch >= '0' && ch <= '9' }
func isAlpha(ch byte) bool { return isLower(ch) || isUpper(ch) }

func (p *parser) primary() (float64, error) {
	ch := p.peek()
	switch {
	case ch == '(':
		p.i++
		v, err := p.expr()
		if err != nil {
			return 0, err
		}
		if !p.op(')', "") {
			return 0, errSyntax
		}
		return v, nil
	case isDigit(ch) || ch == '.':
		return p.number()
	case isAlpha(ch):
		return p.word()
	}
	return 0, errSyntax
}

func (p *parser) number() (float64, error) {
	start := p.i
	for p.i < len(p.s) && (isDigit(p.s[p.i]) || p.s[p.i] == '.' || p.s[p.i] == '_') {
		p.i++
	}
	// 1e3, 2.5E-4
	s := p.s
	if p.i < len(s) && (s[p.i] == 'e' || s[p.i] == 'E') && p.i+1 < len(s) &&
		(isDigit(s[p.i+1]) || ((s[p.i+1] == '-' || s[p.i+1] == '+') && p.i+2 < len(s) && isDigit(s[p.i+2]))) {
		p.i += 2
		for p.i < len(s) && isDigit(s[p.i]) {
			p.i++
		}
	}
	return parseNumber(s[start:p.i])
}

// parseNumber reads a number as Zig's parseFloat does: underscores only
// between digits, as in 1_000.
func parseNumber(s string) (float64, error) {
	if strings.Contains(s, "_") {
		for i := 0; i < len(s); i++ {
			if s[i] == '_' && (i == 0 || i == len(s)-1 || !isDigit(s[i-1]) || !isDigit(s[i+1])) {
				return 0, errSyntax
			}
		}
		s = strings.ReplaceAll(s, "_", "")
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, errSyntax
	}
	return v, nil
}

func (p *parser) word() (float64, error) {
	start := p.i
	for p.i < len(p.s) && isAlpha(p.s[p.i]) {
		p.i++
	}
	w := p.s[start:p.i]
	if strings.EqualFold(w, "pi") {
		return math.Pi, nil
	}
	if strings.EqualFold(w, "e") {
		return math.E, nil
	}
	// Function names are lowercase only, as Zig's stringToEnum.
	switch w {
	case "sqrt", "sin", "cos", "tan", "ln", "log", "abs":
	default:
		return 0, errSyntax
	}
	p.math = true
	if !p.op('(', "") {
		return 0, errSyntax
	}
	x, err := p.expr()
	if err != nil {
		return 0, err
	}
	if !p.op(')', "") {
		return 0, errSyntax
	}
	switch w {
	case "sqrt":
		if x < 0 {
			return 0, errDomain
		}
		return math.Sqrt(x), nil
	case "sin":
		return math.Sin(x), nil
	case "cos":
		return math.Cos(x), nil
	case "tan":
		return math.Tan(x), nil
	case "ln":
		if x <= 0 {
			return 0, errDomain
		}
		return math.Log(x), nil
	case "log":
		if x <= 0 {
			return 0, errDomain
		}
		return math.Log10(x), nil
	}
	return math.Abs(x), nil
}

// Eval returns the value of query and true if it is math. Math that can't
// be computed (1/0, a dangling operator) is false too.
func Eval(query string) (float64, bool) {
	s := strings.Trim(query, " =")
	if s == "" {
		return 0, false
	}
	p := parser{s: s}
	v, err := p.expr()
	if err != nil || p.peek() != 0 || !p.math {
		return 0, false
	}
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, false
	}
	return v, true
}

// FormatNumber writes the value as the row shows it (grouped) and the
// clipboard takes it (not): whole numbers below 1e15 with thousands
// separators ("1,048,576"), others to 10 decimals without trailing zeros.
func FormatNumber(v float64, grouped bool) string {
	if math.Abs(v) < 1e15 && v == math.Round(v) {
		n := int64(v)
		if !grouped {
			return strconv.FormatInt(n, 10)
		}
		d := strconv.FormatUint(uint64(abs64(n)), 10)
		var b strings.Builder
		if n < 0 {
			b.WriteByte('-')
		}
		for i := 0; i < len(d); i++ {
			if i > 0 && (len(d)-i)%3 == 0 {
				b.WriteByte(',')
			}
			b.WriteByte(d[i])
		}
		return b.String()
	}
	s := strconv.FormatFloat(v, 'f', 10, 64)
	if strings.IndexByte(s, '.') >= 0 {
		s = strings.TrimRight(s, "0")
		s = strings.TrimSuffix(s, ".")
	}
	return s
}

func abs64(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}
