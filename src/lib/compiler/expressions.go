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

		switch leftType := left.(type) {
		case *ir.InstLoad, *ir.InstCall:
			if structType, ok := leftType.Type().(*types.PointerType); ok {
				if _, ok := structType.ElemType.(*types.StructType); ok {
					// Check if the class has a method with the name "classname.op.operator"
					methodName := fmt.Sprintf("%s.op.%s", structType.ElemType.Name(), right.Op)
					if method, ok := ctx.lookupFunction(methodName); ok {
						// Call the method and use its result as the result
						left = ctx.NewCall(method, left, rightVal)
						continue
					}
				}
			} else if _, ok := leftType.Type().(*types.StructType); ok {
				// Check if the class has a method with the name "classname.op.operator"
				methodName := fmt.Sprintf("%s.op.%s", leftType.Type().Name(), right.Op)
				if method, ok := ctx.lookupFunction(methodName); ok {
					// Call the method and use its result as the result
					left = ctx.NewCall(method, left, rightVal)
					continue
				}
			} else if _, ok := leftType.Type().(*types.ArrayType); ok {
				// Check if the class has a method with the name "classname.op.operator"
				methodName := fmt.Sprintf("%s.op.%s", leftType.Type().Name(), right.Op)
				if method, ok := ctx.lookupFunction(methodName); ok {
					// Call the method and use its result as the result
					left = ctx.NewCall(method, left, rightVal)
					continue
				}
			} else {
				switch right.Op {
				case "+":
					left = ctx.NewAdd(left, rightVal)
				case "-":
					left = ctx.NewSub(left, rightVal)
				default:
					return nil, posError(right.Pos, "Unknown expression operator: %s", right.Op)
				}
			}
		default:
			switch right.Op {
			case "+":
				left = ctx.NewAdd(left, rightVal)
			case "-":
				left = ctx.NewSub(left, rightVal)
			default:
				return nil, posError(right.Pos, "Unknown expression operator: %s", right.Op)
			}
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
			return nil, posError(right.Pos, "Unknown comparison operator: %s", right.Op)
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
			return nil, posError(right.Pos, "Unknown term operator: %s", right.Op)
		}
	}
	return left, nil
}

func (ctx *Context) compileFactor(f *parser.Factor) (value.Value, error) {
	if f.Value != nil {
		return ctx.compileValue(f.Value)
	} else if f.Identifier != nil {
		val, vType, err := ctx.compileIdentifier(f.Identifier, false)
		if err != nil {
			return nil, err
		}
		if f.GEP != nil {
			gepExpr, err := ctx.compileExpression(f.GEP)
			if err != nil {
				return nil, err
			}
			return ctx.NewGetElementPtr(vType, val, gepExpr), nil
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
		return nil, posError(f.Pos, "Unknown factor type")
	}
}

func (ctx *Context) compileClassInitializer(ci *parser.ClassInitializer) (value.Value, error) {
	// Lookup the class
	class, exists := ctx.lookupClass(ci.ClassName)
	if !exists {
		return nil, posError(ci.Pos, "Class %s not found", ci.ClassName)
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
		return nil, posError(fc.Pos, "Function %s not found", fc.FunctionName)
	}

	// Compile the arguments
	compiledArgs := make([]value.Value, len(fc.Args.Arguments))
	for i, arg := range fc.Args.Arguments {
		if i < len(function.Sig.Params) {
			ctx.RequestedType = function.Sig.Params[i]
		}
		expr, err := ctx.compileExpression(arg)
		if err != nil {
			return nil, err
		}
		ctx.RequestedType = nil
		compiledArgs[i] = expr
	}

	// Call the function
	return ctx.NewCall(function, compiledArgs...), nil
}

func (ctx *Context) compileValue(v *parser.Value) (value.Value, error) {
	if v.Float != nil {
		if ctx.RequestedType != nil {
			if ctx.RequestedType == types.Float {
				return constant.NewFloat(types.Float, *v.Float), nil
			} else if ctx.RequestedType == types.Double {
				return constant.NewFloat(types.Double, *v.Float), nil
			} else {
				return nil, posError(v.Pos, "Cannot convert float to %s", ctx.RequestedType.Name())
			}
		}
		return constant.NewFloat(types.Double, *v.Float), nil
	} else if v.Int != nil {
		if ctx.RequestedType != nil {
			intType, ok := ctx.RequestedType.(*types.IntType)
			if ok {
				return constant.NewInt(intType, *v.Int), nil
			} else {
				return nil, posError(v.Pos, "Cannot convert int to %T", ctx.RequestedType)
			}
		}
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
			return nil, posError(v.Pos, "Error parsing string: %s", err)
		}
		strLen := len(str) + 1
		// Declare malloc if it hasn't been declared yet
		malloc, ok := ctx.lookupFunction("malloc")
		if !ok {
			malloc = ctx.Module.NewFunc("malloc", types.I8Ptr, ir.NewParam("size", types.I64))
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
			return nil, posError(v.Pos, "Unknown duration unit: %s", v.Duration.Unit)
		}
		return constant.NewFloat(types.Double, v.Duration.Number*factor), nil
	} else if v.Null {
		return constant.NewNull(types.I8Ptr), nil
	} else {
		return nil, posError(v.Pos, "Unknown value type")
	}
}

func (ctx *Context) compileIdentifier(i *parser.Identifier, returnTopLevelStruct bool) (value.Value, types.Type, error) {
	val := ctx.lookupVariable(i.Name)
	if val == nil {
		return nil, nil, posError(i.Pos, "Variable %s not found", i.Name)
	}

	if i.Sub == nil {
		return val.Value, val.Type, nil
	}

	// Iterate over the subs
	currentVal := val
	currentSub := i.Sub
	for currentSub != nil {
		fieldType, fieldPtr, isMethod, err := ctx.compileSubIdentifier(currentVal, currentSub)
		if err != nil {
			return nil, nil, err
		}
		if isMethod {
			if returnTopLevelStruct {
				return currentVal.Value, currentVal.Type, nil
			} else {
				return nil, nil, posError(i.Pos, "Cannot call method %s on %s", currentSub.Name, currentVal.Type.Name())
			}
		}

		if currentSub.Sub == nil && !returnTopLevelStruct {
			// If this is the last sub and we're not returning the top-level struct,
			// return the field pointer
			return fieldPtr, fieldPtr.Type(), nil
		}

		// Otherwise, load the field and continue
		currentVal.Value = ctx.NewLoad(fieldType, fieldPtr)
		currentSub = currentSub.Sub
	}

	// If we're here, we're returning the top-level struct
	return currentVal.Value, currentVal.Type, nil
}

func (ctx *Context) compileSubIdentifier(f *Variable, sub *parser.Identifier) (FieldType types.Type, Pointer value.Value, IsMethod bool, err error) {
	if sub != nil {
		_, isMethod := ctx.lookupMethod(f.Type, sub.Name)
		if isMethod {
			return f.Type, f.Value, true, nil
		}

		var field *parser.FieldDefinition
		var nfield int
		elemtypename := f.Value.Type().(*types.PointerType).ElemType.Name()
		if elemtypename == "" {
			elemtypename = f.Type.Name()
		}
		for f := range ctx.Compiler.StructFields[elemtypename] {
			if ctx.Compiler.StructFields[elemtypename][f].Name == sub.Name {
				field = ctx.Compiler.StructFields[elemtypename][f]
				nfield = f
				break
			}
		}
		if field == nil {
			return nil, nil, false, posError(sub.Pos, "Field %s not found in struct %s", sub.Name, elemtypename)
		}
		fieldPtr := ctx.NewGetElementPtr(ctx.stringToType(field.Type), f.Value, constant.NewInt(types.I32, int64(nfield)))
		f.Value = fieldPtr
		return ctx.compileSubIdentifier(f, sub.Sub)
	}
	return f.Type, f.Value, false, nil
}

func (ctx *Context) compileClassMethod(cm *parser.ClassMethod) (value.Value, error) {
	// First, compile the class identifier to get the class instance
	classInstance, _, err := ctx.compileIdentifier(cm.Identifier, true)
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
		return nil, cli.Exit(color.RedString("Error: Cannot call method on non-pointer type"), 1)
	}
	method, exists := ctx.lookupMethod(pointerType, methodName)
	if !exists {
		return nil, cli.Exit(color.RedString("Error: Method %s not found on type %s", methodName, pointerType.ElemType.Name()), 1)
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
