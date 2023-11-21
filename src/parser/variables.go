package parser

import (
	"fmt"

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
		fmt.Printf("Declare variable %s of type %s with value %v\n", name, typeName, value)
		switch typeName {
		case "int":
			statements = append(statements, &compiler.SDefine{Name: name, Typ: types.I64, Expr: value})
		case "string":
			statements = append(statements, &compiler.SDefine{Name: name, Typ: &types.PointerType{ElemType: types.I8}, Expr: value})
		case "float64":
			statements = append(statements, &compiler.SDefine{Name: name, Typ: types.Double, Expr: value})
		case "bool":
			statements = append(statements, &compiler.SDefine{Name: name, Typ: types.I1, Expr: value})
		case "duration":
			statements = append(statements, &compiler.SDefine{Name: name, Typ: types.I64, Expr: value})
		default:
			panic(fmt.Sprintf("Unknown type %s", typeName))
		}
	} else {
		fmt.Printf("Declare variable %s of type %s\n", name, typeName)
		switch typeName {
		case "int":
			statements = append(statements, &compiler.SDefine{Name: name, Typ: types.I64, Expr: nil})
		case "string":
			statements = append(statements, &compiler.SDefine{Name: name, Typ: &types.PointerType{ElemType: types.I8}, Expr: nil})
		case "float64":
			statements = append(statements, &compiler.SDefine{Name: name, Typ: types.Double, Expr: nil})
		case "bool":
			statements = append(statements, &compiler.SDefine{Name: name, Typ: types.I1, Expr: nil})
		case "duration":
			statements = append(statements, &compiler.SDefine{Name: name, Typ: types.I64, Expr: nil})
		default:
			panic(fmt.Sprintf("Unknown type %s", typeName))
		}
	}
	p.Pos++ // ";"
	return statements
}
