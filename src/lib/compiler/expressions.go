package compiler

import (
	"fmt"
	"strconv"

	"github.com/fatih/color"
	"github.com/llir/llvm/ir"
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
			left = ctx.NewAdd(left, rightVal)
		case "-":
			left = ctx.NewSub(left, rightVal)
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
			left = ctx.NewICmp(enum.IPredEQ, left, rightVal)
		case "!=":
			left = ctx.NewICmp(enum.IPredNE, left, rightVal)
		case ">":
			left = ctx.NewICmp(enum.IPredSGT, left, rightVal)
		case "<":
			left = ctx.NewICmp(enum.IPredSLT, left, rightVal)
		case ">=":
			left = ctx.NewICmp(enum.IPredSGE, left, rightVal)
		case "<=":
			left = ctx.NewICmp(enum.IPredSLE, left, rightVal)
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
			left = ctx.NewMul(left, rightVal)
		case "/":
			left = ctx.NewSDiv(left, rightVal)
		case "%":
			left = ctx.NewSRem(left, rightVal)
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
		val, err := ctx.compileIdentifier(f.Identifier, false)
		if err != nil {
			return nil, err
		}
		if v, ok := val.(*ir.InstAlloca); ok {
			return ctx.NewLoad(v.Type().(*types.PointerType).ElemType, val), nil
		} else if v, ok := val.(*ir.InstPhi); ok {
			return ctx.NewLoad(v.Type().(*types.PointerType).ElemType, val), nil
		} else if v, ok := val.(*ir.InstGetElementPtr); ok {
			return ctx.NewLoad(v.Type().(*types.PointerType).ElemType, val), nil
		}
		return val, nil
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
	classPtr := ctx.NewAlloca(class)

	// Initialize the class
	constructor, exists := ctx.lookupFunction(class.Name() + ".constructor")
	if exists {
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
		ctx.NewCall(constructor, append([]value.Value{classPtr}, compiledArgs...)...)
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
	return ctx.NewCall(function, compiledArgs...), nil
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
		mem := ctx.NewCall(malloc, constant.NewInt(types.I64, int64(strLen)))
		// Store the string in the allocated memory
		for i, char := range str {
			ctx.NewStore(constant.NewInt(types.I8, int64(char)), ctx.NewGetElementPtr(types.I8, mem, constant.NewInt(types.I32, int64(i))))
		}
		// Add null character at the end
		ctx.NewStore(constant.NewInt(types.I8, 0), ctx.NewGetElementPtr(types.I8, mem, constant.NewInt(types.I32, int64(len(str)))))
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

func (ctx *Context) compileIdentifier(i *parser.Identifier, returnTopLevelStruct bool) (value.Value, error) {
	val := ctx.lookupVariable(i.Name)
	if val == nil {
		return nil, fmt.Errorf("Variable %s not found", i.Name)
	}

	if i.Sub == nil {
		return val, nil
	}

	// Iterate over the subs
	currentVal := val
	currentSub := i.Sub
	for currentSub != nil {
		fieldType, fieldPtr, isMethod, err := ctx.compileSubIdentifier(currentVal.Type(), currentVal, currentSub)
		if err != nil {
			return nil, err
		}
		if isMethod {
			if returnTopLevelStruct {
				return currentVal, nil
			} else {
				return nil, fmt.Errorf("Unexpected method in identifier")
			}
		}

		if currentSub.Sub == nil && !returnTopLevelStruct {
			// If this is the last sub and we're not returning the top-level struct,
			// return the field pointer
			return fieldPtr, nil
		}

		// Otherwise, load the field and continue
		currentVal = ctx.NewLoad(fieldType, fieldPtr)
		currentSub = currentSub.Sub
	}

	// If we're here, we're returning the top-level struct
	return currentVal, nil
}

func (ctx *Context) compileSubIdentifier(fieldType types.Type, pointer value.Value, sub *parser.Identifier) (FieldType types.Type, Pointer value.Value, IsMethod bool, err error) {
	if sub != nil {
		_, isMethod := ctx.lookupMethod(fieldType, sub.Name)
		if isMethod {
			return fieldType, pointer, true, nil
		}

		var field *parser.FieldDefinition
		var nfield int
		elemtypename := pointer.Type().(*types.PointerType).ElemType.Name()
		for f := range ctx.Compiler.StructFields[elemtypename] {
			if ctx.Compiler.StructFields[elemtypename][f].Name == sub.Name {
				field = ctx.Compiler.StructFields[elemtypename][f]
				nfield = f
				break
			}
		}
		if field == nil {
			return nil, nil, false, cli.Exit(color.RedString("Error: Field %s not found in struct %s", sub.Name, elemtypename), 1)
		}
		fieldPtr := ctx.NewGetElementPtr(stringToType(field.Type), pointer, constant.NewInt(types.I32, int64(nfield)))
		return ctx.compileSubIdentifier(stringToType(field.Type), fieldPtr, sub.Sub)
	}
	return fieldType, pointer, false, nil
}

func (ctx *Context) compileClassMethod(cm *parser.ClassMethod) (value.Value, error) {
	// First, compile the class identifier to get the class instance
	classInstance, err := ctx.compileIdentifier(cm.Identifier, true)
	if err != nil {
		return nil, err
	}

	var methodName string
	var currentSub *parser.Identifier
	for currentSub = cm.Identifier.Sub; currentSub != nil; currentSub = currentSub.Sub {
		methodName = currentSub.Name
	}

	// Then, compile the method call on the class instance
	return ctx.compileMethodCall(classInstance, methodName, cm.Args)
}

func (ctx *Context) compileMethodCall(classInstance value.Value, methodName string, arguments *parser.ArgumentList) (value.Value, error) {
	// Lookup the method on the class
	pointerType, ok := classInstance.Type().(*types.PointerType)
	if !ok {
		return nil, cli.Exit(color.RedString("classInstance is not a pointer type"), 1)
	}
	method, exists := ctx.lookupMethod(pointerType, methodName)
	if !exists {
		return nil, cli.Exit(color.RedString("Error: Method %s not found for class %s", methodName, pointerType.ElemType.Name()), 1)
	}

	// Prepare the arguments for the method call
	args := []value.Value{classInstance}
	for _, arg := range arguments.Arguments {
		compiledArg, err := ctx.compileExpression(arg)
		if err != nil {
			return nil, err
		}
		args = append(args, compiledArg)
	}

	// Call the method
	return ctx.Block.NewCall(method, args...), nil
}

func (ctx *Context) lookupMethod(parentType types.Type, methodName string) (value.Value, bool) {
	// Check if parentType is a pointer to a struct type
	ptrType, ok := parentType.(*types.PointerType)
	if !ok {
		return nil, false
	}
	structType, ok := ptrType.ElemType.(*types.StructType)
	if !ok {
		return nil, false
	}

	// Check if the struct type is a named type
	structName, ok := ctx.structNames[structType]
	if !ok {
		return nil, false
	}

	// Check if methodName is a method of the struct
	methodKey := fmt.Sprintf("%s.%s", structName, methodName)
	method, exists := ctx.SymbolTable[methodKey]
	return method, exists
}
