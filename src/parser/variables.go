package parser

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/llir/llvm/ir/types"
	"github.com/vyPal/CaffeineC/compiler"
)

func (p *Parser) parseVarDecl() []compiler.Stmt {
	p.Pos++ // "var"
	name := p.Tokens[p.Pos].Value
	p.Pos++ // name
	p.Pos++ // ":"
	typeName := p.Tokens[p.Pos].Value
	p.Pos++ // type
	var statements []compiler.Stmt
	if p.Tokens[p.Pos].Type == "PUNCT" && p.Tokens[p.Pos].Value == "=" {
		p.Pos++ // "="
		value := p.parseExpression()
		if p.isBuiltinType(typeName) {
			statements = append(statements, p.createBuiltinTypeStmt(name, typeName, value))
		} else {
			// Assume typeName is a class name
			statements = append(statements, &compiler.SDefine{Name: name, Typ: &compiler.CType{CustomType: typeName}, Expr: value})
		}
	} else {
		if p.isBuiltinType(typeName) {
			statements = append(statements, p.createBuiltinTypeStmt(name, typeName, nil))
		} else {
			// Assume typeName is a class name
			statements = append(statements, &compiler.SDefine{Name: name, Typ: &compiler.CType{CustomType: typeName}, Expr: nil})
		}
	}
	p.Pos++ // ";"
	return statements
}

func (p *Parser) isBuiltinType(typeName string) bool {
	switch typeName {
	case "int", "string", "float64", "bool", "duration":
		return true
	default:
		return false
	}
}

func (p *Parser) createBuiltinTypeStmt(name string, typeName string, value compiler.Expr) compiler.Stmt {
	switch typeName {
	case "int":
		return &compiler.SDefine{Name: name, Typ: &compiler.CType{Typ: types.I8}, Expr: value}
	case "string":
		return &compiler.SDefine{Name: name, Typ: &compiler.CType{Typ: &types.PointerType{ElemType: types.I8}}, Expr: value}
	case "float64":
		return &compiler.SDefine{Name: name, Typ: &compiler.CType{Typ: types.Double}, Expr: value}
	case "bool":
		return &compiler.SDefine{Name: name, Typ: &compiler.CType{Typ: types.I1}, Expr: value}
	case "duration":
		return &compiler.SDefine{Name: name, Typ: &compiler.CType{Typ: types.I64}, Expr: value}
	default:
		color.Red("Unknown type %s", typeName)
		panic(fmt.Sprintf("Unknown type %s", typeName))
	}
}
