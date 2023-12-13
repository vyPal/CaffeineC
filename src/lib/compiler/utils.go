package compiler

import (
	"github.com/llir/llvm/ir/types"
)

func (ctx *Context) stringToType(name string) types.Type {
	switch name {
	// Standard types
	case "int":
		return types.I64
	case "float":
		return types.Double
	case "bool":
		return types.I1
	case "void":
		return types.Void
	case "string":
		return types.NewPointer(types.I8)
	case "duration":
		return types.I64
	// Signed integers (i8, i16, i32, i64)
	case "i8":
		return types.I8
	case "i16":
		return types.I16
	case "i32":
		return types.I32
	case "i64":
		return types.I64
	// Unsigned integers (u8, u16, u32, u64)
	case "u8":
		return types.I8
	case "u16":
		return types.I16
	case "u32":
		return types.I32
	case "u64":
		return types.I64
	// Floating point (f32, f64)
	case "f32":
		return types.Float
	case "f64":
		return types.Double
	// Aliases
	case "byte":
		return types.I8
	case "char":
		return types.I32
	// Pointers
	case "ptr8", "*8", "*i8":
		return types.NewPointer(types.I8)
	case "ptr16", "*16", "*i16":
		return types.NewPointer(types.I16)
	case "ptr32", "*32", "*i32":
		return types.NewPointer(types.I32)
	case "ptr64", "*64", "*i64":
		return types.NewPointer(types.I64)
	default:
		for _, t := range ctx.Module.TypeDefs {
			if t.Name() == name {
				return t
			}
		}
		return types.Void
	}
}
