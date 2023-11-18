package compiler

import (
	"fmt"

	"github.com/llir/llvm/ir/constant"
	"github.com/llir/llvm/ir/types"
	"github.com/llir/llvm/ir/value"
)

func (c *Context) compileFunctionCall(s SFuncCall) value.Value {
	f := c.lookupVariable(s.Name)
	args := make([]value.Value, len(s.Args))
	for i, arg := range s.Args {
		args[i] = c.compileExpr(arg)
	}
	return c.NewCall(f, args...)
}

func (c *Context) compileFunctionCallExpr(e ECall) value.Value {
	f := c.lookupVariable(e.Name)
	args := make([]value.Value, len(e.Args))
	for i, arg := range e.Args {
		args[i] = c.compileExpr(arg)
	}
	return c.NewCall(f, args...)
}

func (c *Context) compileFunctionDecl(s SFuncDecl) {
	f := c.Module.NewFunc(s.Name, s.ReturnType, s.Args...)
	f.Sig.Variadic = true
	f.Sig.RetType = s.ReturnType
	block := f.NewBlock("entry")
	ctx := NewContext(block, c.Compiler)
	for _, stmt := range s.Body {
		ctx.compileStmt(stmt)
	}
	if ctx.Term == nil {
		if s.ReturnType.Equal(types.Void) {
			ctx.NewRet(nil)
		} else {
			panic(fmt.Errorf("function `%s` does not return a value", s.Name))
		}
	}

	c.vars[s.Name] = f
}
func (c *Context) compilePrintCall(s SPrint) {
	// Declare the printf function if it hasn't been declared yet
	printf := c.Context.SymbolTable["printf"]

	// Compile the expression to print
	value := c.compileExpr(s.Expr)

	// If the value is a pointer, load the value it points to
	if ptrType, ok := value.Type().(*types.PointerType); ok {
		value = c.Block.NewLoad(ptrType.ElemType, value)
	}

	// Determine the format string based on the type of the expression
	var formatString string
	switch t := value.Type().(type) {
	case *types.IntType:
		formatString = "%d\n"
	case *types.FloatType:
		formatString = "%f\n"
	case *types.ArrayType: // Assuming strings are represented as an array of characters
		if t.ElemType.Equal(types.I8) {
			fmt.Printf("%T: %v\n", t, t)
			// Create a global constant for the string
			str := c.Module.NewGlobalDef(".str", value.(constant.Constant))
			// Get a pointer to the first element of the string
			value = c.Block.NewGetElementPtr(str.Type(), str, constant.NewInt(types.I32, 0))
			formatString = "%s\n"
		}
	case *types.PointerType:
		if t.ElemType.Equal(types.I8) {
			// Allocate memory for the string
			str := c.Block.NewAlloca(value.Type())
			// Store the string in the allocated memory
			c.Block.NewStore(value, str)
			// Get a pointer to the first element of the string
			value = c.Block.NewLoad(value.Type(), str)
			formatString = "%s\n"
		}
	default:
		panic(fmt.Errorf("cannot print value of type `%s`", value.Type()))
	}

	// Create a global constant for the format string
	format := c.Module.NewGlobalDef(".fmt", constant.NewCharArrayFromString(formatString))

	// Create the call to printf
	c.Block.NewCall(printf, format, value)
}
