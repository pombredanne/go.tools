// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package exact implements mathematically exact values
// and operations for all Go basic types.
//
package exact

import (
	"fmt"
	"go/token"
	"math/big"
	"strconv"
)

// Kind specifies the kind of value represented by a Value.
type Kind int

// Implementation note: Kinds must be enumerated in
// order of increasing "complexity" (used by match).

const (
	// unknown values
	Unknown Kind = iota

	// non-numeric values
	Nil
	Bool
	String

	// numeric values
	Int
	Float
	Complex
)

// A Value represents a mathematically precise value of a given Kind.
type Value interface {
	// Kind returns the value kind; it is always the smallest
	// kind in which the value can be represented exactly.
	Kind() Kind

	// String returns a human-readable form of the value.
	String() string

	// Prevent external implementations.
	implementsValue()
}

// TODO(gri) Handle unknown values more consistently. Some operations
//           accept unknowns, some don't.

// ----------------------------------------------------------------------------
// Implementations

type (
	unknownVal struct{}
	nilVal     struct{}
	boolVal    bool
	stringVal  string
	int64Val   int64
	intVal     struct{ val *big.Int }
	floatVal   struct{ val *big.Rat }
	complexVal struct{ re, im *big.Rat }
)

func (unknownVal) Kind() Kind { return Unknown }
func (nilVal) Kind() Kind     { return Nil }
func (boolVal) Kind() Kind    { return Bool }
func (stringVal) Kind() Kind  { return String }
func (int64Val) Kind() Kind   { return Int }
func (intVal) Kind() Kind     { return Int }
func (floatVal) Kind() Kind   { return Float }
func (complexVal) Kind() Kind { return Complex }

func (unknownVal) String() string   { return "unknown" }
func (nilVal) String() string       { return "nil" }
func (x boolVal) String() string    { return fmt.Sprintf("%v", bool(x)) }
func (x stringVal) String() string  { return strconv.Quote(string(x)) }
func (x int64Val) String() string   { return strconv.FormatInt(int64(x), 10) }
func (x intVal) String() string     { return x.val.String() }
func (x floatVal) String() string   { return x.val.String() }
func (x complexVal) String() string { return fmt.Sprintf("(%s + %si)", x.re, x.im) }

func (unknownVal) implementsValue() {}
func (nilVal) implementsValue()     {}
func (boolVal) implementsValue()    {}
func (stringVal) implementsValue()  {}
func (int64Val) implementsValue()   {}
func (intVal) implementsValue()     {}
func (floatVal) implementsValue()   {}
func (complexVal) implementsValue() {}

// int64 bounds
var (
	minInt64 = big.NewInt(-1 << 63)
	maxInt64 = big.NewInt(1<<63 - 1)
)

func normInt(x *big.Int) Value {
	if minInt64.Cmp(x) <= 0 && x.Cmp(maxInt64) <= 0 {
		return int64Val(x.Int64())
	}
	return intVal{x}
}

func normFloat(x *big.Rat) Value {
	if x.IsInt() {
		return normInt(x.Num())
	}
	return floatVal{x}
}

func normComplex(re, im *big.Rat) Value {
	if im.Sign() == 0 {
		return normFloat(re)
	}
	return complexVal{re, im}
}

// ----------------------------------------------------------------------------
// Factories

// MakeUnknown returns the Unknown value.
func MakeUnknown() Value { return unknownVal{} }

// MakeNil returns the Nil value.
func MakeNil() Value { return nilVal{} }

// MakeBool returns the Bool value for x.
func MakeBool(b bool) Value { return boolVal(b) }

// MakeString returns the String value for x.
func MakeString(s string) Value { return stringVal(s) }

// MakeInt64 returns the Int value for x.
func MakeInt64(x int64) Value { return int64Val(x) }

// MakeUint64 returns the Int value for x.
func MakeUint64(x uint64) Value { return normInt(new(big.Int).SetUint64(x)) }

// MakeFloat64 returns the numeric value for x.
// If x is not finite, the result is unknown.
func MakeFloat64(x float64) Value {
	f := new(big.Rat).SetFloat64(x)
	if f != nil {
		return normFloat(f)
	}
	return unknownVal{}
}

// MakeFromLiteral returns the corresponding literal value.
// If the literal has illegal format, the result is nil.
func MakeFromLiteral(lit string, tok token.Token) Value {
	switch tok {
	case token.INT:
		if x, err := strconv.ParseInt(lit, 0, 64); err == nil {
			return int64Val(x)
		}
		if x, ok := new(big.Int).SetString(lit, 0); ok {
			return intVal{x}
		}

	case token.FLOAT:
		if x, ok := new(big.Rat).SetString(lit); ok {
			return normFloat(x)
		}

	case token.IMAG:
		if n := len(lit); n > 0 && lit[n-1] == 'i' {
			if im, ok := new(big.Rat).SetString(lit[0 : n-1]); ok {
				return normComplex(big.NewRat(0, 1), im)
			}
		}

	case token.CHAR:
		if n := len(lit); n >= 2 {
			if code, _, _, err := strconv.UnquoteChar(lit[1:n-1], '\''); err == nil {
				return int64Val(code)
			}
		}

	case token.STRING:
		if s, err := strconv.Unquote(lit); err == nil {
			return stringVal(s)
		}
	}

	// TODO(gri) should we instead a) return unknown, or b) an error?
	return nil
}

// ----------------------------------------------------------------------------
// Accessors

// BoolVal returns the Go boolean value of x, which must be a Bool.
func BoolVal(x Value) bool { return bool(x.(boolVal)) }

// StringVal returns the Go string value of x, which must be a String.
func StringVal(x Value) string { return string(x.(stringVal)) }

// Int64Val returns the Go int64 value of x and whether the result is exact;
// x must be an Int. If the result is not exact, its value is undefined.
func Int64Val(x Value) (int64, bool) {
	switch x := x.(type) {
	case int64Val:
		return int64(x), true
	case intVal:
		return x.val.Int64(), x.val.BitLen() <= 63
	}
	panic(fmt.Sprintf("invalid Int64Val(%v)", x))
}

// Uint64Val returns the Go uint64 value of x and whether the result is exact;
// x must be an Int. If the result is not exact, its value is undefined.
func Uint64Val(x Value) (uint64, bool) {
	switch x := x.(type) {
	case int64Val:
		return uint64(x), x >= 0
	case intVal:
		return x.val.Uint64(), x.val.Sign() >= 0 && x.val.BitLen() <= 64
	}
	panic(fmt.Sprintf("invalid Uint64Val(%v)", x))
}

// Float64Val returns the nearest Go float64 value of x and whether the result is exact;
// x must be numeric but not Complex.
func Float64Val(x Value) (float64, bool) {
	switch x := x.(type) {
	case int64Val:
		f := float64(int64(x))
		return f, int64Val(f) == x
	case intVal:
		return new(big.Rat).SetFrac(x.val, int1).Float64()
	case floatVal:
		return x.val.Float64()
	}
	panic(fmt.Sprintf("invalid Float64Val(%v)", x))
}

// BitLen() returns the number of bits required to represent
// the absolute value x in binary representation; x must be an Int.
func BitLen(x Value) int {
	switch x := x.(type) {
	case int64Val:
		return new(big.Int).SetInt64(int64(x)).BitLen()
	case intVal:
		return x.val.BitLen()
	}
	panic(fmt.Sprintf("invalid BitLen(%v)", x))
}

// Sign returns -1, 0, or 1 depending on whether
// x < 0, x == 0, or x > 0. For complex values z,
// the sign is 0 if z == 0, otherwise it is != 0.
// For unknown values, the sign is always 1 (this
// helps avoiding "division by zero errors"). For
// all other values, Sign panics.
//
func Sign(x Value) int {
	switch x := x.(type) {
	case unknownVal:
		return 1
	case int64Val:
		switch {
		case x < 0:
			return -1
		case x > 0:
			return 1
		}
		return 0
	case intVal:
		return x.val.Sign()
	case floatVal:
		return x.val.Sign()
	case complexVal:
		return x.re.Sign() | x.im.Sign()
	}
	panic(fmt.Sprintf("invalid Sign(%v)", x))
}

// ----------------------------------------------------------------------------
// Support for assembling/disassembling complex numbers

// MakeImag returns the numeric value x*i (possibly 0);
// x must be numeric but not Complex.
// If x is unknown, the result is unknown.
func MakeImag(x Value) Value {
	var im *big.Rat
	switch x := x.(type) {
	case unknownVal:
		return x
	case int64Val:
		im = big.NewRat(int64(x), 1)
	case intVal:
		im = new(big.Rat).SetFrac(x.val, int1)
	case floatVal:
		im = x.val
	default:
		panic(fmt.Sprintf("invalid MakeImag(%v)", x))
	}
	return normComplex(rat0, im)
}

// Real returns the real part of x, which must be a numeric value.
// If x is unknown, the result is unknown.
func Real(x Value) Value {
	if z, ok := x.(complexVal); ok {
		return normFloat(z.re)
	}
	return x
}

// Imag returns the imaginary part of x, which must be a numeric value.
// If x is unknown, the result is 0.
func Imag(x Value) Value {
	if z, ok := x.(complexVal); ok {
		return normFloat(z.im)
	}
	return int64Val(0)
}

// ----------------------------------------------------------------------------
// Operations

// is32bit reports whether x can be represented using 32 bits.
func is32bit(x int64) bool {
	const s = 32
	return -1<<(s-1) <= x && x <= 1<<(s-1)-1
}

// is63bit reports whether x can be represented using 63 bits.
func is63bit(x int64) bool {
	const s = 63
	return -1<<(s-1) <= x && x <= 1<<(s-1)-1
}

// UnaryOp returns the result of the unary expression op y.
// The operation must be defined for the operand.
// If size >= 0 it specifies the ^ (xor) result size in bytes.
//
func UnaryOp(op token.Token, y Value, size int) Value {
	switch op {
	case token.ADD:
		switch y.(type) {
		case unknownVal, int64Val, intVal, floatVal, complexVal:
			return y
		}

	case token.SUB:
		switch y := y.(type) {
		case int64Val:
			if z := -y; z != y {
				return z // no overflow
			}
			return normInt(new(big.Int).Neg(big.NewInt(int64(y))))
		case intVal:
			return normInt(new(big.Int).Neg(y.val))
		case floatVal:
			return normFloat(new(big.Rat).Neg(y.val))
		case complexVal:
			return normComplex(new(big.Rat).Neg(y.re), new(big.Rat).Neg(y.im))
		}

	case token.XOR:
		var z big.Int
		switch y := y.(type) {
		case int64Val:
			z.Not(big.NewInt(int64(y)))
		case intVal:
			z.Not(y.val)
		default:
			goto Error
		}
		// For unsigned types, the result will be negative and
		// thus "too large": We must limit the result size to
		// the type's size.
		if size >= 0 {
			s := uint(size) * 8
			z.AndNot(&z, new(big.Int).Lsh(big.NewInt(-1), s)) // z &^= (-1)<<s
		}
		return normInt(&z)

	case token.NOT:
		return !y.(boolVal)
	}

Error:
	panic(fmt.Sprintf("invalid unary operation %s%v", op, y))
}

var (
	int1 = big.NewInt(1)
	rat0 = big.NewRat(0, 1)
)

func ord(x Value) int {
	switch x.(type) {
	default:
		return 0
	case int64Val:
		return 1
	case intVal:
		return 2
	case floatVal:
		return 3
	case complexVal:
		return 4
	}
}

// match returns the matching representation (same type) with the
// smallest complexity for two values x and y. If one of them is
// numeric, both of them must be numeric.
//
func match(x, y Value) (_, _ Value) {
	if ord(x) > ord(y) {
		y, x = match(y, x)
		return x, y
	}
	// ord(x) <= ord(y)

	switch x := x.(type) {
	case unknownVal, nilVal, boolVal, stringVal, complexVal:
		return x, y

	case int64Val:
		switch y := y.(type) {
		case int64Val:
			return x, y
		case intVal:
			return intVal{big.NewInt(int64(x))}, y
		case floatVal:
			return floatVal{big.NewRat(int64(x), 1)}, y
		case complexVal:
			return complexVal{big.NewRat(int64(x), 1), rat0}, y
		}

	case intVal:
		switch y := y.(type) {
		case intVal:
			return x, y
		case floatVal:
			return floatVal{new(big.Rat).SetFrac(x.val, int1)}, y
		case complexVal:
			return complexVal{new(big.Rat).SetFrac(x.val, int1), rat0}, y
		}

	case floatVal:
		switch y := y.(type) {
		case floatVal:
			return x, y
		case complexVal:
			return complexVal{x.val, rat0}, y
		}
	}

	panic("unreachable")
}

// BinaryOp returns the result of the binary expression x op y.
// The operation must be defined for the operands.
// To force integer division of Int operands, use op == token.QUO_ASSIGN
// instead of token.QUO; the result is guaranteed to be Int in this case.
// Division by zero leads to a run-time panic.
//
func BinaryOp(x Value, op token.Token, y Value) Value {
	x, y = match(x, y)

	switch x := x.(type) {
	case unknownVal:
		return x

	case boolVal:
		y := y.(boolVal)
		switch op {
		case token.LAND:
			return x && y
		case token.LOR:
			return x || y
		}

	case int64Val:
		a := int64(x)
		b := int64(y.(int64Val))
		var c int64
		switch op {
		case token.ADD:
			if !is63bit(a) || !is63bit(b) {
				return normInt(new(big.Int).Add(big.NewInt(a), big.NewInt(b)))
			}
			c = a + b
		case token.SUB:
			if !is63bit(a) || !is63bit(b) {
				return normInt(new(big.Int).Sub(big.NewInt(a), big.NewInt(b)))
			}
			c = a - b
		case token.MUL:
			if !is32bit(a) || !is32bit(b) {
				return normInt(new(big.Int).Mul(big.NewInt(a), big.NewInt(b)))
			}
			c = a * b
		case token.QUO:
			return normFloat(new(big.Rat).SetFrac(big.NewInt(a), big.NewInt(b)))
		case token.QUO_ASSIGN: // force integer division
			c = a / b
		case token.REM:
			c = a % b
		case token.AND:
			c = a & b
		case token.OR:
			c = a | b
		case token.XOR:
			c = a ^ b
		case token.AND_NOT:
			c = a &^ b
		default:
			goto Error
		}
		return int64Val(c)

	case intVal:
		a := x.val
		b := y.(intVal).val
		var c big.Int
		switch op {
		case token.ADD:
			c.Add(a, b)
		case token.SUB:
			c.Sub(a, b)
		case token.MUL:
			c.Mul(a, b)
		case token.QUO:
			return normFloat(new(big.Rat).SetFrac(a, b))
		case token.QUO_ASSIGN: // force integer division
			c.Quo(a, b)
		case token.REM:
			c.Rem(a, b)
		case token.AND:
			c.And(a, b)
		case token.OR:
			c.Or(a, b)
		case token.XOR:
			c.Xor(a, b)
		case token.AND_NOT:
			c.AndNot(a, b)
		default:
			goto Error
		}
		return normInt(&c)

	case floatVal:
		a := x.val
		b := y.(floatVal).val
		var c big.Rat
		switch op {
		case token.ADD:
			c.Add(a, b)
		case token.SUB:
			c.Sub(a, b)
		case token.MUL:
			c.Mul(a, b)
		case token.QUO:
			c.Quo(a, b)
		default:
			goto Error
		}
		return normFloat(&c)

	case complexVal:
		y := y.(complexVal)
		a, b := x.re, x.im
		c, d := y.re, y.im
		var re, im big.Rat
		switch op {
		case token.ADD:
			// (a+c) + i(b+d)
			re.Add(a, c)
			im.Add(b, d)
		case token.SUB:
			// (a-c) + i(b-d)
			re.Sub(a, c)
			im.Sub(b, d)
		case token.MUL:
			// (ac-bd) + i(bc+ad)
			var ac, bd, bc, ad big.Rat
			ac.Mul(a, c)
			bd.Mul(b, d)
			bc.Mul(b, c)
			ad.Mul(a, d)
			re.Sub(&ac, &bd)
			im.Add(&bc, &ad)
		case token.QUO:
			// (ac+bd)/s + i(bc-ad)/s, with s = cc + dd
			var ac, bd, bc, ad, s big.Rat
			ac.Mul(a, c)
			bd.Mul(b, d)
			bc.Mul(b, c)
			ad.Mul(a, d)
			s.Add(c.Mul(c, c), d.Mul(d, d))
			re.Add(&ac, &bd)
			re.Quo(&re, &s)
			im.Sub(&bc, &ad)
			im.Quo(&im, &s)
		default:
			goto Error
		}
		return normComplex(&re, &im)

	case stringVal:
		if op == token.ADD {
			return x + y.(stringVal)
		}
	}

Error:
	panic(fmt.Sprintf("invalid binary operation %v %s %v", x, op, y))
}

// Shift returns the result of the shift expression x op s
// with op == token.SHL or token.SHR (<< or >>). x must be
// an Int.
//
func Shift(x Value, op token.Token, s uint) Value {
	switch x := x.(type) {
	case unknownVal:
		return x

	case int64Val:
		if s == 0 {
			return x
		}
		switch op {
		case token.SHL:
			z := big.NewInt(int64(x))
			return normInt(z.Lsh(z, s))
		case token.SHR:
			return x >> s
		}

	case intVal:
		if s == 0 {
			return x
		}
		var z big.Int
		switch op {
		case token.SHL:
			return normInt(z.Lsh(x.val, s))
		case token.SHR:
			return normInt(z.Rsh(x.val, s))
		}
	}

	panic(fmt.Sprintf("invalid shift %v %s %d", x, op, s))
}

func cmpZero(x int, op token.Token) bool {
	switch op {
	case token.EQL:
		return x == 0
	case token.NEQ:
		return x != 0
	case token.LSS:
		return x < 0
	case token.LEQ:
		return x <= 0
	case token.GTR:
		return x > 0
	case token.GEQ:
		return x >= 0
	}
	panic("unreachable")
}

// Compare returns the result of the comparison x op y.
// The comparison must be defined for the operands.
//
func Compare(x Value, op token.Token, y Value) bool {
	x, y = match(x, y)

	switch x := x.(type) {
	case unknownVal:
		return true

	case boolVal:
		y := y.(boolVal)
		switch op {
		case token.EQL:
			return x == y
		case token.NEQ:
			return x != y
		}

	case int64Val:
		y := y.(int64Val)
		switch op {
		case token.EQL:
			return x == y
		case token.NEQ:
			return x != y
		case token.LSS:
			return x < y
		case token.LEQ:
			return x <= y
		case token.GTR:
			return x > y
		case token.GEQ:
			return x >= y
		}

	case intVal:
		return cmpZero(x.val.Cmp(y.(intVal).val), op)

	case floatVal:
		return cmpZero(x.val.Cmp(y.(floatVal).val), op)

	case complexVal:
		y := y.(complexVal)
		re := x.re.Cmp(y.re)
		im := x.im.Cmp(y.im)
		switch op {
		case token.EQL:
			return re == 0 && im == 0
		case token.NEQ:
			return re != 0 || im != 0
		}

	case stringVal:
		y := y.(stringVal)
		switch op {
		case token.EQL:
			return x == y
		case token.NEQ:
			return x != y
		case token.LSS:
			return x < y
		case token.LEQ:
			return x <= y
		case token.GTR:
			return x > y
		case token.GEQ:
			return x >= y
		}
	}

	panic(fmt.Sprintf("invalid comparison %v %s %v", x, op, y))
}
