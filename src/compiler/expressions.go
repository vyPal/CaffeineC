package compiler

import (
	"fmt"

	"github.com/llir/llvm/ir"
	"github.com/llir/llvm/ir/constant"
	"github.com/llir/llvm/ir/enum"
	"github.com/llir/llvm/ir/types"
	"github.com/llir/llvm/ir/value"
)

func (ctx *Context) compileExpr(e Expr) value.Value {
	switch e := e.(type) {
	case EConst:
		return ctx.compileConst(e)
	case EVar:
		v := ctx.lookupVariable(e.Name)
		return v
	case EField:
		structVal := ctx.compileExpr(e.Struct)
		//fieldName := ctx.compileExpr(e.Name)
		var field Field
		elemtypename := structVal.Type().(*types.PointerType).ElemType.Name()
		for f := range ctx.Compiler.StructFields[elemtypename] {
			if ctx.Compiler.StructFields[elemtypename][f].Name == e.Name.(EVar).Name {
				field = ctx.Compiler.StructFields[elemtypename][f]
				break
			}
		}
		fieldPtr := ctx.Block.NewGetElementPtr(field.Type, structVal, constant.NewInt(types.I32, int64(field.Index)))
		return ctx.Block.NewLoad(field.Type, fieldPtr)
	case EAssign:
		switch name := e.Name.(type) {
		case EVar:
			v := ctx.compileExpr(e.Value)
			if p, ok := v.Type().(*types.PointerType); ok {
				v = ctx.Block.NewLoad(p.ElemType, v)
			}
			ctx.vars[name.Name] = v
			return v
		case EField:
			structVal := ctx.compileExpr(name.Struct)
			//fieldName := ctx.compileExpr(e.Name)

			var field Field
			elemtypename := structVal.Type().(*types.PointerType).ElemType.Name()
			for f := range ctx.Compiler.StructFields[elemtypename] {
				if ctx.Compiler.StructFields[elemtypename][f].Name == name.Name.(EVar).Name {
					field = ctx.Compiler.StructFields[elemtypename][f]
					break
				}
			}

			// Ensure structVal is a pointer
			if _, ok := structVal.Type().(*types.PointerType); !ok {
				structVal = ctx.Block.NewLoad(types.NewPointer(structVal.Type()), structVal)
			}

			fieldPtr := ctx.Block.NewGetElementPtr(structVal.Type().(*types.PointerType).ElemType, structVal, constant.NewInt(types.I32, int64(field.Index)))

			// Ensure value is of correct type
			if fieldPtr.Type().(*types.PointerType).ElemType != field.Type {
				panic(fmt.Errorf("field type mismatch: expected %s, got %s", field.Type, fieldPtr.Type().(*types.PointerType).ElemType))
			}

			return ctx.Block.NewLoad(fieldPtr.Type().(*types.PointerType).ElemType, fieldPtr)
		default:
			panic(fmt.Errorf("unknown assignment type: %T", name))
		}
	case EClassConstructor:
		// Allocate memory for the struct
		var structType types.Type
		for _, t := range ctx.Module.TypeDefs {
			if t.Name() == e.Name {
				structType = t
				break
			}
		}
		if structType == nil {
			panic(fmt.Sprintf("type `%s` not found", structType))
		}
		fmt.Printf("Type: %T\n", structType)
		structVal := ctx.Block.NewAlloca(structType)
		// Call the constructor
		constructor := ctx.Compiler.SymbolTable[e.Name]
		if constructor == nil {
			panic(fmt.Sprintf("constructor for type `%s` not found", e.Name))
		}
		args := []value.Value{structVal}
		for _, arg := range e.Args {
			args = append(args, ctx.compileExpr(arg))
		}
		ctx.Block.NewCall(constructor, args...)
		return structVal
	case EClassMethod:
		// Get the class instance
		instance := ctx.lookupVariable(e.InstanceName)

		var classType types.Type
		for _, t := range ctx.Module.TypeDefs {
			if t.Equal(instance.Type().(*types.PointerType).ElemType) {
				classType = t
				break
			}
		}
		if classType == nil {
			panic(fmt.Errorf("type `%s` not found", instance.Type().String()))
		}
		// Get the method
		method := ctx.Compiler.SymbolTable[instance.Type().(*types.PointerType).ElemType.Name()+"."+e.MethodName].(*ir.Func)
		if method == nil {
			panic(fmt.Errorf("method '%s.%s' not found", instance.Type().(*types.PointerType).ElemType.Name(), e.MethodName))
		}
		// Get the function arguments
		var args []value.Value
		for _, arg := range e.Args {
			args = append(args, ctx.compileExpr(arg))
		}
		// Call the function
		return ctx.NewCall(method, append([]value.Value{instance}, args...)...)
	case ECall:
		return ctx.compileFunctionCallExpr(e)
	case EAdd:
		l, r := ctx.compileExpr(e.Left), ctx.compileExpr(e.Right)
		if l.Type().Equal(types.NewPointer(types.I64)) {
			l = ctx.NewLoad(types.I64, l)
		}
		if r.Type().Equal(types.NewPointer(types.I64)) {
			r = ctx.NewLoad(types.I64, r)
		}
		return ctx.NewAdd(l, r)
	case ESub:
		l, r := ctx.compileExpr(e.Left), ctx.compileExpr(e.Right)
		return ctx.NewSub(l, r)
	case EMul:
		l, r := ctx.compileExpr(e.Left), ctx.compileExpr(e.Right)
		return ctx.NewMul(l, r)
	case EDiv:
		l, r := ctx.compileExpr(e.Left), ctx.compileExpr(e.Right)
		return ctx.NewFDiv(l, r)
	case EGt:
		l, r := ctx.compileExpr(e.Left), ctx.compileExpr(e.Right)
		return ctx.NewICmp(enum.IPredSGT, l, r)
	case ELt:
		l, r := ctx.compileExpr(e.Left), ctx.compileExpr(e.Right)
		return ctx.NewICmp(enum.IPredSLT, l, r)
	case EEq:
		l, r := ctx.compileExpr(e.Left), ctx.compileExpr(e.Right)
		return ctx.NewICmp(enum.IPredEQ, l, r)
	case ENEq:
		l, r := ctx.compileExpr(e.Left), ctx.compileExpr(e.Right)
		return ctx.NewICmp(enum.IPredNE, l, r)
	case EEGt:
		l, r := ctx.compileExpr(e.Left), ctx.compileExpr(e.Right)
		return ctx.NewICmp(enum.IPredSGE, l, r)
	case EELt:
		l, r := ctx.compileExpr(e.Left), ctx.compileExpr(e.Right)
		return ctx.NewICmp(enum.IPredSLE, l, r)
	case EAnd:
		l, r := ctx.compileExpr(e.Left), ctx.compileExpr(e.Right)
		return ctx.NewAnd(l, r)
	case EOr:
		l, r := ctx.compileExpr(e.Left), ctx.compileExpr(e.Right)
		return ctx.NewOr(l, r)
	case ENot:
		return ctx.NewFNeg(ctx.compileExpr(e.Expr))
	default:
		panic(fmt.Errorf("unknown expression type: %T", e))
	}
}

func (ctx *Context) compileConst(e EConst) value.Value {
	switch e := e.(type) {
	case EVoid:
		return nil
	case EInt:
		if ctx.Compiler.VarsCanBeNumbers {
			value := ctx.lookupVariable(fmt.Sprint(e.Value))
			if value != nil {
				return value
			}
		}
		return constant.NewInt(types.I64, e.Value)
	case EFloat:
		if ctx.Compiler.VarsCanBeNumbers {
			value := ctx.lookupVariable(fmt.Sprint(e.Value))
			if value != nil {
				return value
			}
		}
		return constant.NewFloat(types.Double, e.Value)
	case EBool:
		if ctx.Compiler.VarsCanBeNumbers {
			value := ctx.lookupVariable(fmt.Sprint(e.Value))
			if value != nil {
				return value
			}
		}
		var value int64 = 0
		if e.Value == true {
			value = 1
		}
		return constant.NewInt(types.I1, value)
	case EString:
		str := e.Value
		strLen := len(str) + 1 // +1 for the null terminator
		// Declare malloc if it hasn't been declared yet
		malloc, ok := ctx.Compiler.SymbolTable["malloc"]
		if !ok {
			malloc = ctx.Module.NewFunc("malloc", types.NewPointer(types.I8), ir.NewParam("size", types.I64))
			ctx.Compiler.SymbolTable["malloc"] = malloc
		}
		// Allocate memory for the string
		mem := ctx.Block.NewCall(malloc, constant.NewInt(types.I64, int64(strLen)))
		// Store the string in the allocated memory
		for i, char := range str {
			ctx.Block.NewStore(constant.NewInt(types.I8, int64(char)), ctx.Block.NewGetElementPtr(types.I8, mem, constant.NewInt(types.I32, int64(i))))
		}
		// Add null character at the end
		ctx.Block.NewStore(constant.NewInt(types.I8, 0), ctx.Block.NewGetElementPtr(types.I8, mem, constant.NewInt(types.I32, int64(len(str)))))
		return mem
	case EDuration:
		return constant.NewInt(types.I64, int64(e.Value))
	default:
		panic("unknown constant type")
	}
}

func (ctx *Context) compileStmt(stmt Stmt) {
	switch s := stmt.(type) {
	case *SDefine:
		if ctx.Block == nil {
			panic("cannot declare variable outside of a function")
		}
		if s.Typ == nil && s.CustomTypeName != "" {
			for _, t := range ctx.Module.TypeDefs {
				if t.Name() == s.CustomTypeName {
					s.Typ = types.NewPointer(t)
					break
				}
			}
			if s.Typ == nil {
				panic(fmt.Sprintf("type `%s` not found", s.CustomTypeName))
			}
		}
		value := ctx.compileExpr(s.Expr)
		ctx.vars[s.Name] = value
	case *SAssign:
		switch name := s.Name.(type) {
		case EVar:
			v := ctx.compileExpr(s.Expr)
			if p, ok := v.Type().(*types.PointerType); ok {
				v = ctx.Block.NewLoad(p.ElemType, v)
			}
			ctx.vars[name.Name] = v
		case EField:
			structVal := ctx.compileExpr(name.Struct)
			value := ctx.compileExpr(s.Expr)
			fmt.Println(structVal, value)

			var field Field
			fmt.Println(structVal.Type().(*types.PointerType).ElemType.Name())
			elemtypename := structVal.Type().(*types.PointerType).ElemType.Name()
			for f := range ctx.Compiler.StructFields[elemtypename] {
				if ctx.Compiler.StructFields[elemtypename][f].Name == name.Name.(EVar).Name {
					field = ctx.Compiler.StructFields[elemtypename][f]
					break
				}
			}
			fmt.Println(field)
			fieldPtr := ctx.Block.NewGetElementPtr(field.Type, structVal, constant.NewInt(types.I32, int64(field.Index)))

			// Ensure value is of correct type
			if ctx != nil && ctx.Block != nil && value != nil && field.Type != nil {
				value = ctx.Block.NewBitCast(value, field.Type)
			} else {
				fmt.Println("One of the variables is not initialized")
			}
			fmt.Println(fieldPtr)

			ctx.Block.NewStore(value, fieldPtr)
		default:
			panic(fmt.Errorf("unknown assignment type: %T", name))
		}
	case *SPrint:
		ctx.compilePrintCall(*s)
	case *SSleep:
		ctx.compileSleepCall(*s)
	case *SFuncDecl:
		ctx.compileFunctionDecl(*s)
	case *SFuncCall:
		ctx.compileFunctionCall(*s)
	case *SClassMethod:
		ctx.compileClassMethod(*s)
	case *SRet:
		ctx.NewRet(ctx.compileExpr(s.Val))
	case *SIf:
		thenCtx := ctx.NewContext(ctx.Block.Parent.NewBlock("if.then"))
		for _, stmt := range s.Then {
			thenCtx.compileStmt(stmt)
		}
		elseB := ctx.Block.Parent.NewBlock("if.else")
		elseCtx := ctx.NewContext(elseB)
		for _, stmt := range s.Else {
			elseCtx.compileStmt(stmt)
		}
		leaveB := ctx.Block.Parent.NewBlock("leave.if")
		if thenCtx.Block.Term == nil {
			thenCtx.NewBr(leaveB)
		}
		if elseCtx.Block.Term == nil {
			elseCtx.NewBr(leaveB)
		}
		cond := ctx.compileExpr(s.Cond)
		c, ok := cond.Type().(*types.IntType)
		if !ok {
			panic(fmt.Errorf("expected int type for condition, got %s", cond.Type()))
		} else {
			if c.BitSize != 1 {
				cond = ctx.Block.NewTrunc(cond, types.I1)
			}
		}
		ctx.NewCondBr(cond, thenCtx.Block, elseB)
		ctx.Compiler.Context.Block = leaveB
	case *SWhile:
		condCtx := ctx.NewContext(ctx.Block.Parent.NewBlock("while.loop.cond"))
		ctx.NewBr(condCtx.Block)
		loopCtx := ctx.NewContext(ctx.Block.Parent.NewBlock("while.loop.body"))
		leaveB := ctx.Block.Parent.NewBlock("leave.do.while")
		condCtx.NewCondBr(condCtx.compileExpr(s.Cond), loopCtx.Block, leaveB)
		condCtx.leaveBlock = leaveB
		loopCtx.leaveBlock = leaveB
		for _, stmt := range s.Block {
			loopCtx.compileStmt(stmt)
		}
		loopCtx.NewBr(condCtx.Block)
		ctx.Compiler.Context.Block = leaveB
	case *SFor:
		loopCtx := ctx.NewContext(ctx.Block.Parent.NewBlock("for.loop.body"))
		ctx.NewBr(loopCtx.Block)
		firstAppear := loopCtx.NewPhi(ir.NewIncoming(loopCtx.compileExpr(s.InitExpr), ctx.Block))
		loopCtx.vars[s.InitName] = firstAppear
		step := loopCtx.compileExpr(s.Step)
		firstAppear.Incs = append(firstAppear.Incs, ir.NewIncoming(step, loopCtx.Block))
		loopCtx.vars[s.InitName] = step
		leaveB := ctx.Block.Parent.NewBlock("leave.for.loop")
		loopCtx.leaveBlock = leaveB
		for _, stmt := range s.Block {
			loopCtx.compileStmt(stmt)
		}
		loopCtx.NewCondBr(loopCtx.compileExpr(s.Cond), loopCtx.Block, leaveB)
		ctx.Compiler.Context.Block = leaveB
	case *Class:
		ctx.compileClassDeclaration(s)
	case *SBreak:
		ctx.NewBr(ctx.leaveBlock)
	default:
		panic(fmt.Errorf("unknown statement type: %T", stmt))
	}
}
