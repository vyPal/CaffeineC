package compiler

import (
	"strconv"

	"github.com/fatih/color"
	"github.com/llir/llvm/ir/constant"
	"github.com/llir/llvm/ir/enum"
	"github.com/llir/llvm/ir/types"
	"github.com/llir/llvm/ir/value"
	"github.com/urfave/cli/v2"
	"github.com/vyPal/CaffeineC/lib/parser"
)

func (ctx *Context) compileExpression(e *parser.Expression) (value.Value, error) {
	left, err := ctx.compileComparison(e.Left)
	if err != nil {
		return nil, err
	}
	for _, right := range e.Right {
		rightVal, err := ctx.compileComparison(right.Expression)
		if err != nil {
			return nil, err
		}
		switch right.Op {
		case "+":
			left = ctx.Block.NewAdd(left, rightVal)
		case "-":
			left = ctx.Block.NewSub(left, rightVal)
		default:
			cli.Exit(color.RedString("Error: Unknown expression operator: %s", right.Op), 1)
		}
	}
	return left, nil
}

func (ctx *Context) compileComparison(c *parser.Comparison) (value.Value, error) {
	left, err := ctx.compileTerm(c.Left)
	if err != nil {
		return nil, err
	}
	for _, right := range c.Right {
		rightVal, err := ctx.compileTerm(right.Comparison)
		if err != nil {
			return nil, err
		}
		switch right.Op {
		case "==":
			left = ctx.Block.NewICmp(enum.IPredEQ, left, rightVal)
		case "!=":
			left = ctx.Block.NewICmp(enum.IPredNE, left, rightVal)
		case ">":
			left = ctx.Block.NewICmp(enum.IPredSGT, left, rightVal)
		case "<":
			left = ctx.Block.NewICmp(enum.IPredSLT, left, rightVal)
		case ">=":
			left = ctx.Block.NewICmp(enum.IPredSGE, left, rightVal)
		case "<=":
			left = ctx.Block.NewICmp(enum.IPredSLE, left, rightVal)
		default:
			cli.Exit(color.RedString("Error: Unknown comparison operator: %s", right.Op), 1)
		}
	}
	return left, nil
}

func (ctx *Context) compileTerm(t *parser.Term) (value.Value, error) {
	left, err := ctx.compileFactor(t.Left)
	if err != nil {
		return nil, err
	}
	for _, right := range t.Right {
		rightVal, err := ctx.compileFactor(right.Term)
		if err != nil {
			return nil, err
		}
		switch right.Op {
		case "*":
			left = ctx.Block.NewMul(left, rightVal)
		case "/":
			left = ctx.Block.NewSDiv(left, rightVal)
		case "%":
			left = ctx.Block.NewSRem(left, rightVal)
		default:
			cli.Exit(color.RedString("Error: Unknown term operator: %s", right.Op), 1)
		}
	}
	return left, nil
}

func (ctx *Context) compileFactor(f *parser.Factor) (value.Value, error) {
	if f.Value != nil {
		return ctx.compileValue(f.Value)
	} else if f.Identifier != nil {
		return ctx.compileIdentifier(f.Identifier)
	} else if f.ClassMethod != nil {
		return ctx.compileClassMethod(f.ClassMethod)
	} else if f.FunctionCall != nil {
		return ctx.compileFunctionCall(f.FunctionCall)
	} else if f.SubExpression != nil {
		return ctx.compileExpression(f.SubExpression)
	} else if f.ClassInitializer != nil {
		return ctx.compileClassInitializer(f.ClassInitializer)
	} else {
		return nil, cli.Exit(color.RedString("Error: Unknown factor type"), 1)
	}
}

func (ctx *Context) compileClassInitializer(ci *parser.ClassInitializer) (value.Value, error) {
	// Lookup the class
	class, exists := ctx.lookupClass(ci.ClassName)
	if !exists {
		return nil, cli.Exit(color.RedString("Error: Class %s not found", ci.ClassName), 1)
	}
	class = class.(*types.StructType)

	// Allocate memory for the class
	classPtr := ctx.Block.NewAlloca(class)
	classPtr.SetName(ci.ClassName + "_ptr")

	// Initialize the class
	constructor, exists := ctx.lookupFunction(class.Name() + "_init")
	if exists {
		ctx.Block.NewStore(classPtr, ctx.Block.NewCall(constructor, classPtr))

		// Compile the arguments
		compiledArgs := make([]value.Value, len(ci.Args.Arguments))
		for i, arg := range ci.Args.Arguments {
			expr, err := ctx.compileExpression(arg)
			if err != nil {
				return nil, err
			}
			compiledArgs[i] = expr
		}

		// Call the constructor
		ctx.Block.NewCall(constructor, append([]value.Value{classPtr}, compiledArgs...)...)
	}

	// Return the class pointer
	return classPtr, nil
}

func (ctx *Context) compileFunctionCall(fc *parser.FunctionCall) (value.Value, error) {
	// Lookup the function
	function, exists := ctx.lookupFunction(fc.FunctionName)
	if !exists {
		return nil, cli.Exit(color.RedString("Error: Function %s not found", fc.FunctionName), 1)
	}

	// Compile the arguments
	compiledArgs := make([]value.Value, len(fc.Args.Arguments))
	for i, arg := range fc.Args.Arguments {
		expr, err := ctx.compileExpression(arg)
		if err != nil {
			return nil, err
		}
		compiledArgs[i] = expr
	}

	// Call the function
	return ctx.Block.NewCall(function, compiledArgs...), nil
}

func (ctx *Context) compileValue(v *parser.Value) (value.Value, error) {
	if v.Float != nil {
		return constant.NewFloat(types.Double, *v.Float), nil
	} else if v.Int != nil {
		return constant.NewInt(types.I64, *v.Int), nil
	} else if v.Bool != nil {
		var b int64 = 0
		if *v.Bool {
			b = 1
		}
		return constant.NewInt(types.I1, b), nil
	} else if v.String != nil {
		str, err := strconv.Unquote(*v.String)
		if err != nil {
			cli.Exit(color.RedString("Error: Unable to parse string: %s", *v.String), 1)
		}
		strLen := len(str) + 1
		// Declare malloc if it hasn't been declared yet
		malloc, ok := ctx.lookupFunction("malloc")
		if !ok {
			cli.Exit(color.RedString("Error: malloc function not found"), 1)
		}
		// Allocate memory for the string
		mem := ctx.Block.NewCall(malloc, constant.NewInt(types.I64, int64(strLen)))
		// Store the string in the allocated memory
		for i, char := range str {
			ctx.Block.NewStore(constant.NewInt(types.I8, int64(char)), ctx.Block.NewGetElementPtr(types.I8, mem, constant.NewInt(types.I32, int64(i))))
		}
		// Add null character at the end
		ctx.Block.NewStore(constant.NewInt(types.I8, 0), ctx.Block.NewGetElementPtr(types.I8, mem, constant.NewInt(types.I32, int64(len(str)))))
		return mem, nil
	} else if v.Duration != nil {
		var factor float64
		switch v.Duration.Unit {
		case "h":
			factor = 3600
		case "m":
			factor = 60
		case "s":
			factor = 1
		case "ms":
			factor = 0.001
		case "us":
			factor = 0.000001
		case "ns":
			factor = 0.000000001
		default:
			cli.Exit(color.RedString("Error: Unknown duration unit: %s", v.Duration.Unit), 1)
		}
		return constant.NewFloat(types.Double, v.Duration.Number*factor), nil
	} else {
		return nil, cli.Exit(color.RedString("Error: Unknown value type"), 1)
	}
}

func (ctx *Context) compileIdentifier(i *parser.Identifier) (value.Value, error) {
	val := ctx.lookupVariable(i.Name)
	if i.Sub == nil {
		return val, nil
	}
	fieldType, fieldPtr := ctx.compileSubIdentifier(val.Type(), val, i.Sub)
	return ctx.Block.NewLoad(fieldType, fieldPtr), nil
}

func (ctx *Context) compileSubIdentifier(fieldType types.Type, pointer value.Value, sub *parser.Identifier) (FieldType types.Type, Pointer value.Value) {
	if sub != nil {
		val := ctx.Block.NewLoad(fieldType, pointer)
		var field parser.FieldDefinition
		var nfield int
		elemtypename := val.Type().(*types.PointerType).ElemType.Name()
		for f := range ctx.Compiler.StructFields[elemtypename] {
			if ctx.Compiler.StructFields[elemtypename][f].Name == sub.Name {
				field = ctx.Compiler.StructFields[elemtypename][f]
				nfield = f
				break
			}
		}
		fieldPtr := ctx.Block.NewGetElementPtr(stringToType(field.Type), val, constant.NewInt(types.I32, int64(nfield)))
		return ctx.compileSubIdentifier(stringToType(field.Type), fieldPtr, sub.Sub)
	}
	return fieldType, pointer
}

func (ctx *Context) compileClassMethod(cm *parser.ClassMethod) (value.Value, error) {
	// First, compile the class identifier to get the class instance
	classInstance, err := ctx.compileIdentifier(cm.Identifier)
	if err != nil {
		return nil, err
	}
	// Then, compile the method call on the class instance
	return ctx.compileMethodCall(classInstance, cm.Identifier.Name, cm.Args)
}

func (ctx *Context) compileMethodCall(classInstance value.Value, methodName string, arguments *parser.ArgumentList) (value.Value, error) {
	// Lookup the method on the class
	method, exists := ctx.lookupMethod(classInstance.Type(), methodName)
	if !exists {
		return nil, cli.Exit(color.RedString("Error: Method %s not found for class %s", methodName, classInstance.Type().Name()), 1)
	}

	// Compile the arguments
	compiledArgs := make([]value.Value, len(arguments.Arguments))
	for i, arg := range arguments.Arguments {
		expr, err := ctx.compileExpression(arg)
		if err != nil {
			return nil, err
		}
		compiledArgs[i] = expr
	}

	// Call the method
	return ctx.Block.NewCall(method, compiledArgs...), nil
}

func (ctx *Context) lookupMethod(classType types.Type, methodName string) (value.Value, bool) {
	// TODO: Implement method lookup
	return nil, false
}
