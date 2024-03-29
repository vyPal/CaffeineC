package compiler

import (
	"fmt"
	"strconv"
	"strings"

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
		ctx.RequestedType = left.Type()
		rightVal, err := ctx.compileComparison(right.Expression)
		if err != nil {
			return nil, err
		}

		if !left.Type().Equal(rightVal.Type()) {
			targetType := left.Type()

			// Predefined bitcasts
			switch targetType {
			case types.Double:
				if rightVal.Type().Equal(types.Float) {
					rightVal = ctx.NewFPExt(rightVal, types.Double)
				}
			case types.Float:
				if rightVal.Type().Equal(types.Double) {
					rightVal = ctx.NewFPTrunc(rightVal, types.Float)
				}
			}

			if valType, ok := rightVal.Type().(*types.IntType); ok {
				if target, ok := targetType.(*types.IntType); ok {
					if valType.BitSize < target.BitSize {
						// Extend if valType is smaller than targetType
						rightVal = ctx.NewSExt(rightVal, targetType)
					} else if valType.BitSize > target.BitSize {
						// Truncate if valType is larger than targetType
						rightVal = ctx.NewTrunc(rightVal, targetType)
					}
				} else if targetType == types.Float {
					rightVal = ctx.NewSIToFP(rightVal, targetType)
				} else if targetType == types.Double {
					rightVal = ctx.NewSIToFP(rightVal, targetType)
				} else if targetType == types.Half {
					rightVal = ctx.NewSIToFP(rightVal, targetType)
				} else if targetType == types.FP128 {
					rightVal = ctx.NewSIToFP(rightVal, targetType)
				}
			}

			// If the value is a struct type or a pointer to a struct type, try to find a conversion function
			if structType, ok := rightVal.Type().(*types.StructType); ok {
				method, ok := ctx.lookupFunction(structType.Name() + ".get." + targetType.Name())
				if ok {
					// If a conversion function is found, call it and return the result
					rightVal = ctx.NewCall(method, rightVal)
				}
			} else if ptrType, ok := rightVal.Type().(*types.PointerType); ok {
				if structType, ok := ptrType.ElemType.(*types.StructType); ok {
					method, ok := ctx.lookupFunction(structType.Name() + ".get." + targetType.Name())
					if ok {
						// If a conversion function is found, call it and return the result
						rightVal = ctx.NewCall(method, rightVal)
					}
				}
			}

			if !rightVal.Type().Equal(targetType) {
				return nil, posError(right.Pos, "Automated conversion from %s to %s failed.", rightVal.Type(), targetType)
			}
		}

		switch leftType := left.(type) {
		case *ir.InstLoad, *ir.InstCall, *ir.InstAlloca:
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
			}
		}
		switch rightVal.Type() {
		case types.Float:
			if !left.Type().Equal(types.Float) {
				leftFloat := ctx.NewSIToFP(left, rightVal.Type())
				left = leftFloat
			}

			switch right.Op {
			case "+":
				left = ctx.NewFAdd(left, rightVal)
			case "-":
				left = ctx.NewFSub(left, rightVal)
			case "&&":
				left = ctx.NewAnd(left, rightVal)
			case "||":
				left = ctx.NewOr(left, rightVal)
			default:
				return nil, posError(right.Pos, "Unknown expression operator: %s", right.Op)
			}
		case types.Double:
			if !left.Type().Equal(types.Double) {
				leftFloat := ctx.NewSIToFP(left, rightVal.Type())
				left = leftFloat
			}

			switch right.Op {
			case "+":
				left = ctx.NewFAdd(left, rightVal)
			case "-":
				left = ctx.NewFSub(left, rightVal)
			case "&&":
				left = ctx.NewAnd(left, rightVal)
			case "||":
				left = ctx.NewOr(left, rightVal)
			default:
				return nil, posError(right.Pos, "Unknown expression operator: %s", right.Op)
			}
		default:
			switch right.Op {
			case "+":
				left = ctx.NewAdd(left, rightVal)
			case "-":
				left = ctx.NewSub(left, rightVal)
			case "&&":
				left = ctx.NewAnd(left, rightVal)
			case "||":
				left = ctx.NewOr(left, rightVal)
			default:
				return nil, posError(right.Pos, "Unknown expression operator: %s", right.Op)
			}
		}
	}
	ctx.RequestedType = nil
	return left, nil
}

func (ctx *Context) compileComparison(c *parser.Comparison) (value.Value, error) {
	left, err := ctx.compileTerm(c.Left)
	if err != nil {
		return nil, err
	}
	for _, right := range c.Right {
		ctx.RequestedType = left.Type()
		rightVal, err := ctx.compileTerm(right.Comparison)
		if err != nil {
			return nil, err
		}

		if !left.Type().Equal(rightVal.Type()) {
			targetType := left.Type()

			// Predefined bitcasts
			switch targetType {
			case types.Double:
				if rightVal.Type().Equal(types.Float) {
					rightVal = ctx.NewFPExt(rightVal, types.Double)
				}
			case types.Float:
				if rightVal.Type().Equal(types.Double) {
					rightVal = ctx.NewFPTrunc(rightVal, types.Float)
				}
			}

			if valType, ok := rightVal.Type().(*types.IntType); ok {
				if target, ok := targetType.(*types.IntType); ok {
					if valType.BitSize < target.BitSize {
						// Extend if valType is smaller than targetType
						rightVal = ctx.NewSExt(rightVal, targetType)
					} else if valType.BitSize > target.BitSize {
						// Truncate if valType is larger than targetType
						rightVal = ctx.NewTrunc(rightVal, targetType)
					}
				} else if targetType == types.Float {
					rightVal = ctx.NewSIToFP(rightVal, targetType)
				} else if targetType == types.Double {
					rightVal = ctx.NewSIToFP(rightVal, targetType)
				} else if targetType == types.Half {
					rightVal = ctx.NewSIToFP(rightVal, targetType)
				} else if targetType == types.FP128 {
					rightVal = ctx.NewSIToFP(rightVal, targetType)
				}
			}

			// If the value is a struct type or a pointer to a struct type, try to find a conversion function
			if structType, ok := rightVal.Type().(*types.StructType); ok {
				method, ok := ctx.lookupFunction(structType.Name() + ".get." + targetType.Name())
				if ok {
					// If a conversion function is found, call it and return the result
					rightVal = ctx.NewCall(method, rightVal)
				}
			} else if ptrType, ok := rightVal.Type().(*types.PointerType); ok {
				if structType, ok := ptrType.ElemType.(*types.StructType); ok {
					method, ok := ctx.lookupFunction(structType.Name() + ".get." + targetType.Name())
					if ok {
						// If a conversion function is found, call it and return the result
						rightVal = ctx.NewCall(method, rightVal)
					}
				}
			}

			if !rightVal.Type().Equal(targetType) {
				return nil, posError(right.Pos, "Automated conversion from %s to %s failed.", rightVal.Type(), targetType)
			}
		}

		switch leftType := left.(type) {
		case *ir.InstLoad, *ir.InstCall, *ir.InstAlloca:
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
			}
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
	ctx.RequestedType = nil
	return left, nil
}

func (ctx *Context) compileTerm(t *parser.Term) (value.Value, error) {
	left, err := ctx.compileFactor(t.Left)
	if err != nil {
		return nil, err
	}
	for _, right := range t.Right {
		ctx.RequestedType = left.Type()
		rightVal, err := ctx.compileFactor(right.Term)
		if err != nil {
			return nil, err
		}

		if !left.Type().Equal(rightVal.Type()) {
			targetType := left.Type()

			// Predefined bitcasts
			switch targetType {
			case types.Double:
				if rightVal.Type().Equal(types.Float) {
					rightVal = ctx.NewFPExt(rightVal, types.Double)
				}
			case types.Float:
				if rightVal.Type().Equal(types.Double) {
					rightVal = ctx.NewFPTrunc(rightVal, types.Float)
				}
			}

			if valType, ok := rightVal.Type().(*types.IntType); ok {
				if target, ok := targetType.(*types.IntType); ok {
					if valType.BitSize < target.BitSize {
						// Extend if valType is smaller than targetType
						rightVal = ctx.NewSExt(rightVal, targetType)
					} else if valType.BitSize > target.BitSize {
						// Truncate if valType is larger than targetType
						rightVal = ctx.NewTrunc(rightVal, targetType)
					}
				} else if targetType == types.Float {
					rightVal = ctx.NewSIToFP(rightVal, targetType)
				} else if targetType == types.Double {
					rightVal = ctx.NewSIToFP(rightVal, targetType)
				} else if targetType == types.Half {
					rightVal = ctx.NewSIToFP(rightVal, targetType)
				} else if targetType == types.FP128 {
					rightVal = ctx.NewSIToFP(rightVal, targetType)
				}
			}

			// If the value is a struct type or a pointer to a struct type, try to find a conversion function
			if structType, ok := rightVal.Type().(*types.StructType); ok {
				method, ok := ctx.lookupFunction(structType.Name() + ".get." + targetType.Name())
				if ok {
					// If a conversion function is found, call it and return the result
					rightVal = ctx.NewCall(method, rightVal)
				}
			} else if ptrType, ok := rightVal.Type().(*types.PointerType); ok {
				if structType, ok := ptrType.ElemType.(*types.StructType); ok {
					method, ok := ctx.lookupFunction(structType.Name() + ".get." + targetType.Name())
					if ok {
						// If a conversion function is found, call it and return the result
						rightVal = ctx.NewCall(method, rightVal)
					}
				}
			}

			if !rightVal.Type().Equal(targetType) {
				return nil, posError(right.Pos, "Automated conversion from %s to %s failed.", rightVal.Type(), targetType)
			}
		}

		switch leftType := left.(type) {
		case *ir.InstLoad, *ir.InstCall, *ir.InstAlloca:
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
			}
		}

		switch right.Op {
		case "*":
			if types.IsFloat(left.Type()) {
				left = ctx.NewFMul(left, rightVal)
			} else {
				left = ctx.NewMul(left, rightVal)
			}
		case "/":
			if types.IsFloat(left.Type()) {
				left = ctx.NewFDiv(left, rightVal)
			} else {
				left = ctx.NewSDiv(left, rightVal)
			}
		case "%":
			if types.IsFloat(left.Type()) {
				left = ctx.NewFRem(left, rightVal)
			} else {
				left = ctx.NewSRem(left, rightVal)
			}
		default:
			return nil, posError(right.Pos, "Unknown term operator: %s", right.Op)
		}
	}
	ctx.RequestedType = nil
	return left, nil
}

func (ctx *Context) compileFactor(f *parser.Factor) (value.Value, error) {
	if f.Value != nil {
		return ctx.compileValue(f.Value)
	} else if f.Identifier != nil {
		val, _, err := ctx.compileIdentifier(f.Identifier, false)
		if err != nil {
			return nil, err
		}
		if v, ok := val.(*ir.InstAlloca); ok {
			elemType := v.Type().(*types.PointerType).ElemType
			if _, isStruct := elemType.(*types.StructType); isStruct {
				return val, nil
			}
			if _, isPointer := elemType.(*types.PointerType); isPointer {
				return val, nil
			}
			return ctx.NewLoad(elemType, val), nil
		} else if v, ok := val.(*ir.InstPhi); ok {
			return ctx.NewLoad(v.Type().(*types.PointerType).ElemType, val), nil
		} else if v, ok := val.(*ir.InstGetElementPtr); ok {
			return ctx.NewLoad(v.Type().(*types.PointerType).ElemType, val), nil
		}
		return val, nil
	} else if f.BitCast != nil {
		return ctx.compileBitCast(f.BitCast)
	} else if f.ClassMethod != nil {
		return ctx.compileClassMethod(f.ClassMethod)
	} else if f.FunctionCall != nil {
		return ctx.compileFunctionCall(f.FunctionCall)
	} else if f.ClassInitializer != nil {
		return ctx.compileClassInitializer(f.ClassInitializer)
	} else {
		return nil, posError(f.Pos, "Unknown factor type")
	}
}

func (ctx *Context) compileBitCast(bc *parser.BitCast) (value.Value, error) {
	val, err := ctx.compileExpression(bc.Expr)
	if err != nil {
		return nil, err
	}

	if bc.Type == "" {
		return val, nil
	}

	targetType := ctx.StringToType(bc.Type)

	// If the value is already of the target type, just return it
	if val.Type().Equal(targetType) {
		return val, nil
	}

	// Predefined bitcasts
	switch targetType {
	case types.Double:
		if val.Type().Equal(types.Float) {
			val = ctx.NewFPExt(val, types.Double)
		}
	case types.Float:
		if val.Type().Equal(types.Double) {
			val = ctx.NewFPTrunc(val, types.Float)
		}
	}

	if valType, ok := val.Type().(*types.IntType); ok {
		if targetType, ok := targetType.(*types.IntType); ok {
			if valType.BitSize < targetType.BitSize {
				// Extend if valType is smaller than targetType
				val = ctx.NewSExt(val, targetType)
			} else if valType.BitSize > targetType.BitSize {
				// Truncate if valType is larger than targetType
				val = ctx.NewTrunc(val, targetType)
			}
		}
	}

	// If the value is a struct type or a pointer to a struct type, try to find a conversion function
	if structType, ok := val.Type().(*types.StructType); ok {
		method, ok := ctx.lookupFunction(structType.Name() + ".get." + bc.Type)
		if ok {
			// If a conversion function is found, call it and return the result
			result := ctx.NewCall(method, val)
			return result, nil
		}
	} else if ptrType, ok := val.Type().(*types.PointerType); ok {
		if structType, ok := ptrType.ElemType.(*types.StructType); ok {
			method, ok := ctx.lookupFunction(structType.Name() + ".get." + bc.Type)
			if ok {
				// If a conversion function is found, call it and return the result
				result := ctx.NewCall(method, val)
				return result, nil
			}
		}
	}

	// If the value is not a custom type or there is no conversion function, perform a bitcast
	bitcast := ctx.NewBitCast(val, targetType)
	if bitcast.Type().Equal(targetType) {
		return bitcast, nil
	}

	return nil, posError(bc.Pos, "Cannot convert %s to %s", val.Type().Name(), bc.Type)
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
		if len(ci.Args.Arguments) != len(constructor.Sig.Params)-1 {
			return nil, posError(ci.Pos, "Invalid number of arguments for class constructor")
		}
		// Compile the arguments
		compiledArgs := make([]value.Value, len(ci.Args.Arguments))
		for i, arg := range ci.Args.Arguments {
			ctx.RequestedType = constructor.Sig.Params[i+1]
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
	fc.FunctionName = strings.Trim(fc.FunctionName, "\"")
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
			if ptrType, ok := ctx.RequestedType.(*types.PointerType); ok {
				local := ctx.NewAlloca(ptrType.ElemType.(*types.FloatType))
				ctx.NewStore(constant.NewFloat(ptrType.ElemType.(*types.FloatType), *v.Float), local)
				return local, nil
			} else if ctx.RequestedType == types.Float {
				return constant.NewFloat(types.Float, *v.Float), nil
			} else if ctx.RequestedType == types.Double {
				return constant.NewFloat(types.Double, *v.Float), nil
			} else if ctx.RequestedType == types.Half {
				return constant.NewFloat(types.Half, *v.Float), nil
			} else if ctx.RequestedType == types.FP128 {
				return constant.NewFloat(types.FP128, *v.Float), nil
			} else if ctx.RequestedType == types.I32 {
				return constant.NewInt(types.I32, int64(*v.Float)), nil
			} else if ctx.RequestedType == types.I64 {
				return constant.NewInt(types.I64, int64(*v.Float)), nil
			} else if ctx.RequestedType == types.I1 {
				if *v.Float == 0 {
					return constant.NewInt(types.I1, 0), nil
				} else {
					return constant.NewInt(types.I1, 1), nil
				}
			} else {
				return nil, posError(v.Pos, "Cannot convert float to %s", ctx.RequestedType.Name())
			}
		}
		return constant.NewFloat(types.Double, *v.Float), nil
	} else if v.Int != nil {
		if ctx.RequestedType != nil {
			if ptrType, ok := ctx.RequestedType.(*types.PointerType); ok {
				local := ctx.NewAlloca(ptrType.ElemType.(*types.IntType))
				ctx.NewStore(constant.NewInt(ptrType.ElemType.(*types.IntType), *v.Int), local)
				return local, nil
			} else if intType, ok := ctx.RequestedType.(*types.IntType); ok {
				return constant.NewInt(intType, *v.Int), nil
			} else if ctx.RequestedType == types.Float {
				return constant.NewFloat(types.Float, float64(*v.Int)), nil
			} else if ctx.RequestedType == types.Double {
				return constant.NewFloat(types.Double, float64(*v.Int)), nil
			} else if ctx.RequestedType == types.Half {
				return constant.NewFloat(types.Half, float64(*v.Int)), nil
			} else if ctx.RequestedType == types.FP128 {
				return constant.NewFloat(types.FP128, float64(*v.Int)), nil
			} else {
				return nil, posError(v.Pos, "Cannot convert int to %T", ctx.RequestedType)
			}
		}
		return constant.NewInt(types.I64, *v.Int), nil
	} else if v.HexInt != nil {
		hexInt, err := strconv.ParseInt(*v.HexInt, 16, 64)
		if err != nil {
			return nil, posError(v.Pos, "Error parsing hex int: %s", err)
		}
		return constant.NewInt(types.I64, hexInt), nil
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
		strGlobal := ctx.Module.NewGlobalDef("", constant.NewCharArrayFromString(str+"\000"))
		strGlobal.Immutable = true
		strGlobal.Linkage = enum.LinkagePrivate
		return strGlobal, nil
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
		if i.GEP != nil {
			ctx.RequestedType = types.I32
			gepExpr, err := ctx.compileExpression(i.GEP)
			if err != nil {
				return nil, nil, err
			}
			ctx.RequestedType = nil

			// Run GetElementPtr on the loaded value
			v := ctx.NewGetElementPtr(val.Type.(*types.PointerType).ElemType, val.Value, gepExpr)
			return v, v.Type(), nil
		}
		// Handle referencing
		for j := 0; j < len(i.Ref); j++ {
			// Create a pointer to the variable
			ptrType := types.NewPointer(val.Value.Type())
			ptr := ctx.NewAlloca(ptrType)
			ctx.NewStore(val.Value, ptr)
			val.Value = ptr
			val.Type = ptrType
		}

		// Handle dereferencing
		for j := 0; j < len(i.Deref); j++ {
			// Load the value the pointer points to
			val.Value = ctx.NewLoad(val.Value.Type().(*types.PointerType).ElemType, val.Value)
			val.Type = val.Value.Type()
		}
		return val.Value, val.Value.Type(), nil
	}

	originalVal := val

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

		// Update currentSub to its subfield
		nextSub := currentSub.Sub

		if nextSub == nil && !returnTopLevelStruct {
			// If this is the last sub and we're not returning the top-level struct,
			// return the field pointer
			// Handle referencing
			for j := 0; j < len(i.Ref); j++ {
				// Create a pointer to the variable
				ptrType := types.NewPointer(fieldPtr.Type())
				ptr := ctx.NewAlloca(ptrType)
				ctx.NewStore(fieldPtr, ptr)
				fieldPtr = ptr
			}

			// Handle dereferencing
			for j := 0; j < len(i.Deref); j++ {
				// Load the value the pointer points to
				fieldPtr = ctx.NewLoad(fieldPtr.Type().(*types.PointerType).ElemType, fieldPtr)
			}
			return fieldPtr, fieldPtr.Type(), nil
		}

		// Otherwise, load the field and continue
		currentVal.Value = ctx.NewLoad(fieldType, fieldPtr)
		currentSub = nextSub
	}

	// Handle referencing
	for j := 0; j < len(i.Ref); j++ {
		// Create a pointer to the variable
		ptrType := types.NewPointer(originalVal.Value.Type())
		ptr := ctx.NewAlloca(ptrType)
		ctx.NewStore(originalVal.Value, ptr)
		originalVal.Value = ptr
		originalVal.Type = ptrType
	}

	// Handle dereferencing
	for j := 0; j < len(i.Deref); j++ {
		// Load the value the pointer points to
		originalVal.Value = ctx.NewLoad(originalVal.Value.Type().(*types.PointerType).ElemType, originalVal.Value)
		originalVal.Type = originalVal.Value.Type()
	}

	// If we're here, we're returning the top-level struct
	return originalVal.Value, originalVal.Type, nil
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

		fieldPtr := ctx.NewGetElementPtr(ctx.StringToType(field.Type), f.Value, constant.NewInt(types.I32, int64(nfield)))
		if sub.GEP != nil {
			ctx.RequestedType = types.I32
			gepExpr, err := ctx.compileExpression(sub.GEP)
			if err != nil {
				return nil, nil, false, err
			}
			ctx.RequestedType = nil

			// Load the array pointer
			arrayPtr := ctx.NewLoad(fieldPtr.Type().(*types.PointerType).ElemType, fieldPtr)

			// Get the pointer to the specific element
			elemPtr := ctx.NewGetElementPtr(arrayPtr.Type().(*types.PointerType).ElemType, arrayPtr, gepExpr)

			return elemPtr.Type().(*types.PointerType).ElemType.(*types.PointerType).ElemType, elemPtr, false, nil
		}
		return ctx.compileSubIdentifier(&Variable{Value: fieldPtr, Type: fieldPtr.Type()}, sub.Sub)
	}
	return f.Type, f.Value, false, nil
}

func (ctx *Context) compileClassMethod(cm *parser.ClassMethod) (value.Value, error) {
	var methodName string
	var currentSub *parser.Identifier
	var prevSub *parser.Identifier

	// Go through the recursive list of identifier.sub
	for currentSub = cm.Identifier.Sub; currentSub != nil; currentSub = currentSub.Sub {
		methodName = currentSub.Name
		if currentSub.Sub != nil {
			prevSub = currentSub
		}
	}

	// Remove the last sub from cm.Identifier
	if prevSub != nil {
		prevSub.Sub = nil
	} else {
		cm.Identifier.Sub = nil
	}

	// Compile the class identifier to get the class instance
	classInstance, _, err := ctx.compileIdentifier(cm.Identifier, true)
	if err != nil {
		return nil, err
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
	structName, ok := ctx.Context.structNames[structType]
	if !ok {
		return nil, false
	}

	// Check if methodName is a method of the struct
	methodKey := fmt.Sprintf("%s.%s", structName, methodName)
	method, exists := ctx.SymbolTable[methodKey]
	return method, exists
}
