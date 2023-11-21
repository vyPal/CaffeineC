package compiler

import (
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
type EAssign struct {
	Expr
	Name  string
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
	EDiv
	Left  Expr
	Right Expr
}
type EEGt struct {
	EDiv
	Left  Expr
	Right Expr
}
type EELt struct {
	EDiv
	Left  Expr
	Right Expr
}
type ELt struct {
	EDiv
	Left  Expr
	Right Expr
}
type EEq struct {
	EDiv
	Left  Expr
	Right Expr
}
type ENEq struct {
	EDiv
	Left  Expr
	Right Expr
}

type Stmt interface{ isStmt() Stmt }
type SDefine struct {
	Stmt
	Name string
	Typ  types.Type
	Expr Expr
}
type SAssign struct {
	Stmt
	Name string
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
type SFuncDecl struct {
	Stmt
	Name       string
	Args       []*ir.Param
	ReturnType types.Type
	Body       []Stmt
}
type SRet struct {
	Stmt
	Val Expr
}

type SIf struct {
	Stmt
	Cond Expr
	Then []Stmt
	Else []Stmt
}
