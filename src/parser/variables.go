package parser

import (
	"fmt"

	"github.com/llir/llvm/ir/constant"
	"github.com/llir/llvm/ir/types"
	"github.com/llir/llvm/ir/value"
)

func (p *Parser) parseVarDecl() {
	p.Pos++ // "var"
	name := p.Tokens[p.Pos].Value
	p.Pos++ // name
	p.Pos++ // ":"
	typeName := p.Tokens[p.Pos].Value
	p.Pos++ // type
	if p.Tokens[p.Pos].Type == "PUNCT" && p.Tokens[p.Pos].Value == "=" {
		p.Pos++ // "="
		value := p.parseExpression()
		fmt.Printf("Declare variable %s of type %s with value %v\n", name, typeName, value)
		switch typeName {
		case "int", "string", "float64", "duration":
			p.Module.NewGlobalDef(name, value.(constant.Constant))
		default:
			panic(fmt.Sprintf("Unknown type %s", typeName))
		}
		p.defineVariable(name, value)
	} else {
		fmt.Printf("Declare variable %s of type %s\n", name, typeName)
		switch typeName {
		case "int", "string", "duration":
			p.Module.NewGlobalDef(name, constant.NewZeroInitializer(types.I64))
		default:
			panic(fmt.Sprintf("Unknown type %s", typeName))
		}
		p.defineVariable(name, constant.NewZeroInitializer(types.I64))
	}
	p.Pos++ // ";"
}

func (p *Parser) defineVariable(name string, val value.Value) {
	p.SymbolTable[name] = val
}
