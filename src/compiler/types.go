package compiler

import (
	"fmt"
	"time"

	"github.com/llir/llvm/ir"
	"github.com/llir/llvm/ir/types"
)

type Expr interface {
	isExpr() Expr
}

type EConst interface {
	Expr
	isEConst() EConst
}

// Type definitions
type EVoid struct{ EConst }
type EInt struct {
	EConst
	Value int64
}
type EFloat struct {
	EConst
	Value float64
}
type EBool struct {
	EConst
	Value bool
}
type EString struct {
	EConst
	Value string
}
type EDuration struct {
	EConst
	Value time.Duration
}

// Expression definitions
type EVar struct {
	Expr
	Name string
}
type EClassConstructor struct {
	Expr
	Name string
	Args []Expr
}
type SClassMethod struct {
	Stmt
	InstanceName string
	MethodName   string
	Args         []Expr
}
type EClassMethod struct {
	Expr
	InstanceName string
	MethodName   string
	Args         []Expr
}
type EClassField struct {
	Expr
	ClassName string
	FieldName string
}
type EField struct {
	Expr
	Struct Expr
	Name   Expr
}
type EAssign struct {
	Expr
	Name  Expr
	Value Expr
}
type ECall struct {
	Expr
	Name string
	Args []Expr
}
type EAdd struct {
	Expr
	Left  Expr
	Right Expr
}
type ESub struct {
	Expr
	Left  Expr
	Right Expr
}
type EMul struct {
	Expr
	Left  Expr
	Right Expr
}
type EDiv struct {
	Expr
	Left  Expr
	Right Expr
}
type EGt struct {
	Expr
	Left  Expr
	Right Expr
}
type EEGt struct {
	Expr
	Left  Expr
	Right Expr
}
type EELt struct {
	Expr
	Left  Expr
	Right Expr
}
type ELt struct {
	Expr
	Left  Expr
	Right Expr
}
type EEq struct {
	Expr
	Left  Expr
	Right Expr
}
type ENEq struct {
	Expr
	Left  Expr
	Right Expr
}
type EAnd struct {
	Expr
	Left  Expr
	Right Expr
}
type EOr struct {
	Expr
	Left  Expr
	Right Expr
}
type ENot struct {
	Expr
	Value Expr
}

type Stmt interface{ isStmt() Stmt }
type SDefine struct {
	Stmt
	Name string
	Typ  *CType
	Expr Expr
}
type SAssign struct {
	Stmt
	Name Expr
	Expr Expr
}
type SPrint struct {
	Stmt
	Expr Expr
}
type SSleep struct {
	Stmt
	Expr Expr
}
type SFuncCall struct {
	Stmt
	Name string
	Args []Expr
}
type CParam struct {
	Name string
	Typ  CType
}

func (ctx *Context) toParam(p *CParam) *ir.Param {
	ctx.toType(&p.Typ)
	return ir.NewParam(p.Name, p.Typ.Typ)
}
func (ctx *Context) toParams(ps []*CParam) []*ir.Param {
	var params []*ir.Param
	for _, p := range ps {
		params = append(params, ctx.toParam(p))
	}
	return params
}

type CType struct {
	Typ        types.Type
	CustomType string
}

func (ctx *Context) toType(t *CType) types.Type {
	if t.CustomType != "" {
		for _, ty := range ctx.Module.TypeDefs {
			if ty.Name() == t.CustomType {
				t.Typ = types.NewPointer(ty)
				break
			}
		}
		if t.Typ == nil {
			panic(fmt.Sprintf("type `%s` not found", t.CustomType))
		}
		return t.Typ
	} else {
		return t.Typ
	}
}

type SFuncDecl struct {
	Stmt
	Name       string
	Args       []*CParam
	ReturnType *CType
	Body       []Stmt
}
type SRet struct {
	Stmt
	Val Expr
}
type SBreak struct {
	Stmt
}

type SIf struct {
	Stmt
	Cond Expr
	Then []Stmt
	Else []Stmt
}

type SWhile struct {
	Stmt
	Cond  Expr
	Block []Stmt
}

type SFor struct {
	Stmt
	InitName string
	InitExpr Expr
	Step     Expr
	Cond     Expr
	Block    []Stmt
}
