package parser

import (
	"fmt"

	"github.com/llir/llvm/ir/types"
	"github.com/vyPal/CaffeineC/compiler"
)

func (p *Parser) parseFunctionDeclaration() compiler.Stmt {
	p.Pos++ // "func"
	name := p.Tokens[p.Pos].Value
	fmt.Println("Func name:", name)
	p.Pos++ // name
	p.Pos++ // "("
	// Parse the parameters
	var params []*compiler.CParam
	for p.Tokens[p.Pos].Type != "PUNCT" || p.Tokens[p.Pos].Value != ")" {
		paramName := p.Tokens[p.Pos].Value
		p.Pos++ // name
		p.Pos++ // ":"
		paramType := p.Tokens[p.Pos].Value
		p.Pos++ // type
		switch paramType {
		case "int":
			params = append(params, &compiler.CParam{Name: paramName, Typ: compiler.CType{Typ: types.I64}})
		case "string":
			params = append(params, &compiler.CParam{Name: paramName, Typ: compiler.CType{Typ: &types.PointerType{ElemType: types.I8}}})
		case "float64":
			params = append(params, &compiler.CParam{Name: paramName, Typ: compiler.CType{Typ: types.Double}})
		case "bool":
			params = append(params, &compiler.CParam{Name: paramName, Typ: compiler.CType{Typ: types.I1}})
		case "duration":
			params = append(params, &compiler.CParam{Name: paramName, Typ: compiler.CType{Typ: types.I64}})
		default:
			params = append(params, &compiler.CParam{Name: paramName, Typ: compiler.CType{CustomType: paramType}})
		}
		if p.Tokens[p.Pos].Type == "PUNCT" && p.Tokens[p.Pos].Value == "," {
			p.Pos++ // ","
		}
	}
	p.Pos++ // ")"
	// Check if the function returns a value
	var returnType *compiler.CType
	if p.Tokens[p.Pos].Type == "PUNCT" && p.Tokens[p.Pos].Value == ":" {
		p.Pos++ // ":"
		switch p.Tokens[p.Pos].Value {
		case "int":
			returnType = &compiler.CType{Typ: types.I64}
		case "string":
			returnType = &compiler.CType{Typ: &types.PointerType{ElemType: types.I8}}
		case "float64":
			returnType = &compiler.CType{Typ: types.Double}
		case "bool":
			returnType = &compiler.CType{Typ: types.I1}
		case "duration":
			returnType = &compiler.CType{Typ: types.I64}
		default:
			returnType = &compiler.CType{CustomType: p.Tokens[p.Pos].Value}
		}
	} else {
		returnType = &compiler.CType{Typ: types.Void}
	}
	p.Pos++ // type
	fmt.Printf("Declare function %s with return type %s\n", name, returnType)
	if returnType.Typ != types.Void {
		p.Pos++ // "{"
	}
	fmt.Println("At cpost:", p.Tokens[p.Pos])
	fmt.Println("Start of function", name)
	var body []compiler.Stmt
	for p.Tokens[p.Pos].Type != "PUNCT" || p.Tokens[p.Pos].Value != "}" {
		token := p.Tokens[p.Pos]
		switch token.Type {
		case "IDENT":
			if token.Value == "var" {
				body = append(body, p.parseVarDecl()...)
			} else if token.Value == "return" {
				p.Pos++ // "return"
				value := p.parseExpression()
				fmt.Println("Return", value)
				body = append(body, &compiler.SRet{Val: value})
			} else if token.Value == "if" {
				body = append(body, p.parseIf())
			} else if token.Value == "print" {
				body = append(body, p.parsePrint())
			} else if token.Value == "sleep" {
				body = append(body, p.parseSleep())
			} else if token.Value == "while" {
				body = append(body, p.parseWhile())
			} else if token.Value == "for" {
				body = append(body, p.parseFor())
			} else if token.Value == "func" {
				body = append(body, p.parseFunctionDeclaration())
			} else if token.Value == "class" {
				body = append(body, p.parseClassDefinition())
			} else if p.Tokens[p.Pos+1].Type == "PUNCT" && p.Tokens[p.Pos+1].Value == "(" {
				body = append(body, p.parseFunctionCall())
			} else if p.Tokens[p.Pos+1].Type == "PUNCT" && p.Tokens[p.Pos+1].Value == "." {
				if p.Tokens[p.Pos+3].Type == "PUNCT" && p.Tokens[p.Pos+3].Value == "(" {
					body = append(body, p.parseMethodCall())
				} else {
					body = append(body, p.parseAssignment())
				}
			} else {
				body = append(body, p.parseAssignment())
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
