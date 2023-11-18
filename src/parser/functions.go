package parser

import (
	"fmt"

	"github.com/llir/llvm/ir"
	"github.com/llir/llvm/ir/types"
	"github.com/vyPal/CaffeineC/compiler"
)

func (p *Parser) parseFunctionDeclaration() compiler.Stmt {
	p.Pos++ // "func"
	name := p.Tokens[p.Pos].Value
	p.Pos++ // name
	p.Pos++ // "("
	// Parse the parameters
	var params []*ir.Param
	for p.Tokens[p.Pos].Type != "PUNCT" || p.Tokens[p.Pos].Value != ")" {
		paramName := p.Tokens[p.Pos].Value
		p.Pos++ // name
		p.Pos++ // ":"
		paramType := p.Tokens[p.Pos].Value
		p.Pos++ // type
		switch paramType {
		case "int":
			params = append(params, ir.NewParam(paramName, types.I64))
		case "string":
			params = append(params, ir.NewParam(paramName, &types.ArrayType{ElemType: types.I8}))
		case "float64":
			params = append(params, ir.NewParam(paramName, types.Double))
		case "duration":
			params = append(params, ir.NewParam(paramName, types.I64))
		default:
			panic(fmt.Sprintf("Unknown type %s", paramType))
		}
		if p.Tokens[p.Pos].Type == "PUNCT" && p.Tokens[p.Pos].Value == "," {
			p.Pos++ // ","
		}
	}
	p.Pos++ // ")"
	// Check if the function returns a value
	var returnType types.Type
	if p.Tokens[p.Pos].Type == "PUNCT" && p.Tokens[p.Pos].Value == ":" {
		p.Pos++ // ":"
		switch p.Tokens[p.Pos].Value {
		case "int":
			returnType = types.I64
		case "string":
			returnType = &types.PointerType{ElemType: types.I8}
		case "float64":
			returnType = types.Double
		case "duration":
			returnType = types.I64
		default:
			panic(fmt.Sprintf("Unknown type %s", p.Tokens[p.Pos].Value))
		}
	} else {
		returnType = types.Void
	}
	p.Pos++ // type
	fmt.Printf("Declare function %s with return type %s\n", name, returnType)
	if returnType != types.Void {
		p.Pos++ // "{"
	}
	fmt.Println("Start of function", name)
	var body []compiler.Stmt
	for p.Tokens[p.Pos].Type != "PUNCT" || p.Tokens[p.Pos].Value != "}" {
		token := p.Tokens[p.Pos]
		switch token.Type {
		case "IDENT":
			if token.Value == "return" {
				p.Pos++ // "return"
				value := p.parseExpression()
				fmt.Println("Return", value)
				body = append(body, &compiler.SRet{Val: value})
			} else if token.Value == "print" {
				body = append(body, p.parsePrint())
			} else if token.Value == "sleep" {
				body = append(body, p.parseSleep())
			} else if token.Value == "func" {
				body = append(body, p.parseFunctionDeclaration())
			} else {
				fmt.Println("[W]", token.Location, "Unexpected identifier:", token.Value)
				p.Pos++
			}
		default:
			fmt.Println("[W]", token.Location, "Unexpected token:", token.Value)
			p.Pos++
		}
	}
	fmt.Println("End of function", name)
	p.Pos++ // "}"
	return &compiler.SFuncDecl{Name: name, Args: params, ReturnType: returnType, Body: body}
}

func (p *Parser) parsePrint() compiler.Stmt {
	p.Pos++ // "print"
	val := p.parseExpression()
	fmt.Println("Print", val)

	p.Pos++ // ";"
	return &compiler.SPrint{Expr: val}
}

func (p *Parser) parseSleep() compiler.Stmt {
	p.Pos++ // "sleep"
	value := p.parseExpression()
	fmt.Println("Sleep", value)

	p.Pos++ // ";"
	return &compiler.SSleep{Expr: value}
}

func (p *Parser) parseFunctionCall() compiler.Stmt {
	// Parse the function name
	name := p.Tokens[p.Pos].Value
	p.Pos++

	// Parse the argument list
	var args []compiler.Expr
	p.Pos++ // "("
	for p.Tokens[p.Pos].Type != "PUNCT" || p.Tokens[p.Pos].Value != ")" {
		args = append(args, p.parseExpression())
		if p.Tokens[p.Pos].Type == "PUNCT" && p.Tokens[p.Pos].Value == "," {
			p.Pos++ // ","
		}
	}
	p.Pos++ // ")"
	fmt.Println("Call", name, args)

	p.Pos++ // ";"
	return &compiler.SFuncCall{Name: name, Args: args}
}

func (p *Parser) parseNonVoidFunctionCall() compiler.Expr {
	// Parse the function name
	name := p.Tokens[p.Pos].Value
	p.Pos++

	// Parse the argument list
	var args []compiler.Expr
	p.Pos++ // "("
	for p.Tokens[p.Pos].Type != "PUNCT" || p.Tokens[p.Pos].Value != ")" {
		args = append(args, p.parseExpression())
		if p.Tokens[p.Pos].Type == "PUNCT" && p.Tokens[p.Pos].Value == "," {
			p.Pos++ // ","
		}
	}
	p.Pos++ // ")"
	fmt.Println("Call", name, args)

	return compiler.ECall{Name: name, Args: args}
}
