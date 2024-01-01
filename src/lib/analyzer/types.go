package analyzer

import (
	"fmt"
	"strconv"
	"strings"
)

type Type interface {
	Name() string
	Equals(Type) bool
}

type IntType struct {
	Base   int
	Signed bool
}

func (t IntType) Name() string {
	return fmt.Sprintf("int%d", t.Base)
}

func (t IntType) Equals(other Type) bool {
	if other, ok := other.(IntType); ok {
		return t.Base == other.Base && t.Signed == other.Signed
	}
	return false
}

type FloatType struct {
	Base int
}

func (t FloatType) Name() string {
	return fmt.Sprintf("float%d", t.Base)
}

func (t FloatType) Equals(other Type) bool {
	if other, ok := other.(FloatType); ok {
		return t.Base == other.Base
	}
	return false
}

type PointerType struct {
	To Type
}

func (t PointerType) Name() string {
	return fmt.Sprintf("%s*", t.To.Name())
}

func (t PointerType) Equals(other Type) bool {
	if other, ok := other.(PointerType); ok {
		return t.To.Equals(other.To)
	}
	return false
}

type CustomType struct {
	CName string
}

func (t CustomType) Name() string {
	return t.CName
}

func (t CustomType) Equals(other Type) bool {
	if other, ok := other.(CustomType); ok {
		return t.CName == other.CName
	}
	return false
}

func NewIntType(signed bool, base int) Type {
	return IntType{Base: base, Signed: signed}
}

func NewFloatType(base int) Type {
	return FloatType{Base: base}
}

func NewPointerType(to Type) Type {
	return PointerType{To: to}
}

func NewCustomType(name string) Type {
	return CustomType{CName: name}
}

func stringToType(s string) Type {
	// Count the number of '*' at the start of the string
	pointerCount := strings.LastIndex(s, "*") + 1

	// Remove the '*' from the start of the string
	s = s[pointerCount:]

	// Determine the base type
	var baseType Type
	if strings.HasPrefix(s, "i") {
		// Signed integer type
		base, _ := strconv.Atoi(s[1:])
		baseType = NewIntType(true, base)
	} else if strings.HasPrefix(s, "u") {
		// Unsigned integer type
		base, _ := strconv.Atoi(s[1:])
		baseType = NewIntType(false, base)
	} else if strings.HasPrefix(s, "f") {
		// Floating point type
		base, _ := strconv.Atoi(s[1:])
		if base == 16 || base == 32 || base == 64 || base == 128 {
			baseType = NewFloatType(base)
		} else {
			return nil // or return an error
		}
	} else {
		// Custom type
		baseType = NewCustomType(s)
	}

	// If the type is a pointer, wrap it in a PointerType
	for i := 0; i < pointerCount; i++ {
		baseType = NewPointerType(baseType)
	}

	return baseType
}
