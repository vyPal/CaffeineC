package compiler

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/alecthomas/participle/v2/lexer"
	"github.com/fatih/color"
	"github.com/llir/llvm/ir/types"
	"github.com/urfave/cli/v2"
)

func posError(pos lexer.Position, message string, args ...interface{}) error {
	return cli.Exit(color.RedString("%s at %s:%d:%d", fmt.Sprintf(message, args...), pos.Filename, pos.Line, pos.Column), 1)
}

func (ctx *Context) StringToType(name string) types.Type {
	pointerCount := strings.Count(name, "*")
	name = strings.TrimLeft(name, "*")

	var typ types.Type
	if strings.HasPrefix(name, "i") || strings.HasPrefix(name, "u") {
		size, _ := strconv.Atoi(name[1:])
		typ = types.NewInt(uint64(size))
	} else {
		switch name {
		case "void", "":
			typ = types.Void
		case "f16":
			typ = types.Half
		case "f32":
			typ = types.Float
		case "f64":
			typ = types.Double
		case "f128":
			typ = types.FP128
		default:
			for _, t := range ctx.Module.TypeDefs {
				if t.Name() == name {
					typ = t
					break
				}
			}
		}
	}

	if typ == nil {
		panic("Unknown type: " + name)
	}

	// If the type is a pointer, wrap it in the appropriate number of pointer types
	for i := 0; i < pointerCount; i++ {
		typ = types.NewPointer(typ)
	}

	return typ
}

func (ctx *Context) TypeToString(typ types.Type) string {
	switch typ := typ.(type) {
	case *types.VoidType:
		return "void"
	case *types.IntType:
		return "i" + strconv.Itoa(int(typ.BitSize))
	case *types.FloatType:
		switch typ.Kind {
		case types.FloatKindHalf:
			return "f16"
		case types.FloatKindFloat:
			return "f32"
		case types.FloatKindDouble:
			return "f64"
		case types.FloatKindFP128:
			return "f128"
		default:
			panic("Unknown float type")
		}
	case *types.PointerType:
		return "*" + ctx.TypeToString(typ.ElemType)
	case *types.StructType:
		return typ.Name()
	default:
		panic("Unknown type")
	}
}
