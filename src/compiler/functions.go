package compiler

import (
	"fmt"
	"strings"

	"github.com/llir/llvm/ir"
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
	for name := range c.usedVars {
		variable := c.lookupVariable(name)
		if _, ok := variable.(*ir.Func); !ok {
			// If it's not a function, check if it's a pointer
			if _, ok := variable.Type().(*types.PointerType); ok {
				args = append(args, variable)
			} else {
				// If it's not a pointer, load the value of the variable
				value := c.Block.NewLoad(variable.Type(), variable)
				args = append(args, value)
			}
		}
	}
	return c.NewCall(f, args...)
}

func (c *Context) compileFunctionCallExpr(e ECall) value.Value {
	f := c.lookupVariable(e.Name)
	args := make([]value.Value, len(e.Args))
	for i, arg := range e.Args {
		args[i] = c.compileExpr(arg)
	}
	for name := range c.usedVars {
		variable := c.lookupVariable(name)
		if _, ok := variable.(*ir.Func); !ok {
			// If it's not a function, check if it's a pointer
			if _, ok := variable.Type().(*types.PointerType); ok {
				// If it's a pointer, load the value it points to
				args = append(args, variable)
			} else {
				// If it's not a pointer, load the value of the variable
				value := c.Block.NewLoad(variable.Type(), variable)
				args = append(args, value)
			}
		}
	}
	return c.NewCall(f, args...)
}

func (c *Context) compileFunctionDecl(s SFuncDecl) {
	// Create a temporary context and block for analysis
	tmpBlock := c.Module.NewFunc("tmp", types.Void)
	tmpCtx := c.NewContext(tmpBlock.NewBlock("entry"))

	var argsUsed []string
	for _, arg := range s.Args {
		argsUsed = append(argsUsed, arg.Name())
		tmpCtx.vars[arg.Name()] = constant.NewInt(types.I1, 0)
		fmt.Println("Defined " + arg.Name())
	}
	for _, stmt := range s.Body {
		tmpCtx.compileStmt(stmt)
	}
	for name := range tmpCtx.usedVars {
		for _, arg := range argsUsed {
			if arg == name {
				continue
			}
		}
		fmt.Println("Used var:", name)
		c.usedVars[name] = true
		value := tmpCtx.lookupVariable(name)
		s.Args = append(s.Args, ir.NewParam("", value.Type()))
	}

	// Remove the temporary function from the module
	c.Module.Funcs = c.Module.Funcs[:len(c.Module.Funcs)-1]

	f := c.Module.NewFunc(s.Name, s.ReturnType, s.Args...)
	f.Sig.Variadic = false
	f.Sig.RetType = s.ReturnType
	block := f.NewBlock("entry")
	ctx := c.NewContext(block)
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
		if ptrType.ElemType.Equal(types.I8Ptr) {
			value = c.Block.NewLoad(ptrType.ElemType, value)
		}
	}

	// Determine the format string based on the type of the expression
	var formatString string
	switch t := value.Type().(type) {
	case *types.IntType:
		formatString = "%d"
	case *types.FloatType:
		formatString = "%f"
	case *types.ArrayType: // Assuming strings are represented as an array of characters
		if t.ElemType.Equal(types.I8) {
			// Create a global constant for the string
			str := c.Module.NewGlobalDef(".str", value.(constant.Constant))
			// Get a pointer to the first element of the string
			value = c.Block.NewGetElementPtr(str.Type(), str, constant.NewInt(types.I32, 0))
			formatString = "%s"
		} else {
			panic(fmt.Errorf("cannot print value of type `%s`", value.Type()))
		}
	case *types.PointerType:
		if t.ElemType.Equal(types.I8) {
			// Allocate memory for the string
			str := c.Block.NewAlloca(value.Type())
			// Store the string in the allocated memory
			c.Block.NewStore(value, str)
			// Get a pointer to the first element of the string
			value = c.Block.NewLoad(value.Type(), str)
			formatString = "%s"
		} else if t.ElemType.Equal(types.I64) {
			// Get a pointer to the first element of the string
			value = c.Block.NewLoad(value.Type(), value)
			formatString = "%d"
		} else {
			panic(fmt.Errorf("cannot print value of type `%s`", value.Type()))
		}
	default:
		panic(fmt.Errorf("cannot print value of type `%s`", value.Type()))
	}

	//Check in c.Module.Globals for an existing format string for this type
	shortFormatString := strings.ReplaceAll(formatString[:len(formatString)-1], "%", "")
	var format *ir.Global
	for _, global := range c.Module.Globals {
		if global.Name() == ".fmt-"+shortFormatString {
			format = global
			break
		}
	}
	if format == nil {
		// Create a global constant for the format string
		format = c.Module.NewGlobalDef(".fmt-"+shortFormatString, constant.NewCharArrayFromString(formatString))
	}

	// Create the call to printf and ignore the return value
	_ = c.Block.NewCall(printf, format, value)
}

func (c *Context) compileSleepCall(s SSleep) {
	// Declare the sleep function if it hasn't been declared yet
	sleep := c.Context.SymbolTable["sleep_ns"]

	// Compile the expression to print
	value := c.compileExpr(s.Expr)

	// Create the call to sleep and ignore the return value
	_ = c.Block.NewCall(sleep, value)
}
