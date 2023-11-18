package compiler

import (
	"fmt"

	"github.com/llir/llvm/ir"
	"github.com/llir/llvm/ir/constant"
	"github.com/llir/llvm/ir/types"
	"github.com/llir/llvm/ir/value"
)

func (c *Context) compileFunctionCall(s SFuncCall) value.Value {
	fn := c.lookupVariable(s.Name)
	entryBlock := fn.(*ir.Func).Blocks[0]
	declCtx := c.NewContext(entryBlock, c.Compiler)
	f := declCtx.lookupVariable(s.Name)
	args := make([]value.Value, len(s.Args))
	for i, arg := range s.Args {
		args[i] = c.compileExpr(arg)
	}
	for name := range declCtx.usedVars {
		variable := declCtx.lookupVariable(name)
		if _, ok := variable.(*ir.Func); !ok {
			// If it's not a function, load the value of the variable
			value := declCtx.Block.NewLoad(variable.Type(), variable)
			args = append(args, value)
		}
	}
	return c.NewCall(f, args...)
}

func (c *Context) compileFunctionCallExpr(e ECall) value.Value {
	fn := c.lookupVariable(e.Name)
	entryBlock := fn.(*ir.Func).Blocks[0]
	declCtx := c.NewContext(entryBlock, c.Compiler)
	f := declCtx.lookupVariable(e.Name)
	args := make([]value.Value, len(e.Args))
	for i, arg := range e.Args {
		args[i] = c.compileExpr(arg)
	}
	for name := range declCtx.usedVars {
		variable := declCtx.lookupVariable(name)
		if _, ok := variable.(*ir.Func); !ok {
			// If it's not a function, load the value of the variable
			value := declCtx.Block.NewLoad(variable.Type(), variable)
			args = append(args, value)
		}
	}
	return c.NewCall(f, args...)
}

func (c *Context) compileFunctionDecl(s SFuncDecl) {
	// Create a temporary context and block for analysis
	tmpFunctions := c.Module.Funcs
	tmpBlock := c.Module.NewFunc("tmp", types.Void)
	tmpCtx := c.NewContext(tmpBlock.NewBlock("entry"), c.Compiler)

	for _, stmt := range s.Body {
		tmpCtx.compileStmt(stmt)
	}
	for name := range tmpCtx.usedVars {
		fmt.Println("Used var:", name)
		c.usedVars[name] = true
		value := tmpCtx.lookupVariable(name)
		s.Args = append(s.Args, ir.NewParam("", value.Type()))
	}

	// Remove the temporary function from the module
	c.Module.Funcs = tmpFunctions

	f := c.Module.NewFunc(s.Name, s.ReturnType, s.Args...)
	f.Sig.Variadic = false
	f.Sig.RetType = s.ReturnType
	block := f.NewBlock("entry")
	ctx := c.NewContext(block, c.Compiler)
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
