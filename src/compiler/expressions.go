package compiler

import (
	"fmt"

	"github.com/llir/llvm/ir"
	"github.com/llir/llvm/ir/constant"
	"github.com/llir/llvm/ir/types"
	"github.com/llir/llvm/ir/value"
)

func (ctx *Context) compileExpr(e Expr) value.Value {
	switch e := e.(type) {
	case EConst:
		return ctx.compileConst(e)
	case EVar:
		return ctx.lookupVariable(e.Name)
	case EAssign:
		v := ctx.compileExpr(e.Value)
		ctx.vars[e.Name] = v
		return v
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
	default:
		panic(fmt.Errorf("unknown expression type: %T", e))
	}
}

func (ctx *Context) compileConst(e EConst) value.Value {
	switch e := e.(type) {
	case EVoid:
		return nil
	case EInt:
		return constant.NewInt(types.I64, e.Value)
	case EFloat:
		return constant.NewFloat(types.Double, e.Value)
	case EString:
		str := e.Value
		strLen := len(str) + 1 // +1 for the null terminator
		// Declare malloc if it hasn't been declared yet
		malloc, ok := ctx.Compiler.SymbolTable["malloc"]
		if !ok {
			malloc = ctx.Module.NewFunc("malloc", types.NewPointer(types.I8), ir.NewParam("size", types.I64))
			ctx.Context.SymbolTable["malloc"] = malloc
		}
		// Allocate memory for the string
		mem := ctx.Block.NewCall(malloc, constant.NewInt(types.I64, int64(strLen)))
		// Store the string in the allocated memory
		for i, char := range str {
			ctx.Block.NewStore(constant.NewInt(types.I8, int64(char)), ctx.Block.NewGetElementPtr(types.I8, mem, constant.NewInt(types.I32, int64(i))))
		}
		// Store the null terminator
		ctx.Block.NewStore(constant.NewInt(types.I8, 0), ctx.Block.NewGetElementPtr(types.I8, mem, constant.NewInt(types.I32, int64(strLen-1))))
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
		v := ctx.NewAlloca(s.Typ)
		value := ctx.compileExpr(s.Expr)
		if value.Type().Equal(types.NewPointer(s.Typ)) {
			value = ctx.NewLoad(s.Typ, value)
		}
		ctx.NewStore(value, v)
		ctx.vars[s.Name] = v
	case *SAssign:
		v := ctx.lookupVariable(s.Name)
		ctx.NewStore(ctx.compileExpr(s.Expr), v)
	case *SPrint:
		ctx.compilePrintCall(*s)
	case *SSleep:
		panic(fmt.Errorf("not implemented: %T", stmt))
	case *SFuncDecl:
		ctx.compileFunctionDecl(*s)
	case *SFuncCall:
		ctx.compileFunctionCall(*s)
	case *SRet:
		ctx.NewRet(ctx.compileExpr(s.Val))
	default:
		panic(fmt.Errorf("unknown statement type: %T", stmt))
	}
}
