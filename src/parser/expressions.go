package parser

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/vyPal/CaffeineC/compiler"
)

func (p *Parser) parseStatement() []compiler.Stmt {
	token := p.Tokens[p.Pos]
	var statements []compiler.Stmt
	switch token.Type {
	case "IDENT":
		if token.Value == "var" {
			statements = append(statements, p.parseVarDecl()...)
		} else if token.Value == "if" {
			statements = append(statements, p.parseIf())
		} else if token.Value == "print" {
			statements = append(statements, p.parsePrint())
		} else if token.Value == "sleep" {
			statements = append(statements, p.parseSleep())
		} else if token.Value == "func" {
			statements = append(statements, p.parseFunctionDeclaration())
		} else if p.Tokens[p.Pos+1].Type == "PUNCT" && p.Tokens[p.Pos+1].Value == "(" {
			statements = append(statements, p.parseFunctionCall())
		} else {
			fmt.Println("[W]", token.Location, "Unexpected identifier:", token.Value)
			p.Pos++
		}
	default:
		fmt.Println("[W]", token.Location, "Unexpected token:", token.Value)
		p.Pos++
	}
	return statements
}

func (p *Parser) parseIf() *compiler.SIf {
	p.Pos++ // "if"
	condition := p.parseExpression()
	p.Pos++ // "{"
	var body []compiler.Stmt
	for p.Tokens[p.Pos].Type != "PUNCT" || p.Tokens[p.Pos].Value != "}" {
		body = append(body, p.parseStatement()...)
	}
	p.Pos++ // "}"
	if p.Tokens[p.Pos].Type == "IDENT" && p.Tokens[p.Pos].Value == "else" {
		p.Pos++ // "else"
		p.Pos++ // "{"
		var elseBody []compiler.Stmt
		for p.Tokens[p.Pos].Type != "PUNCT" || p.Tokens[p.Pos].Value != "}" {
			elseBody = append(elseBody, p.parseStatement()...)
		}
		p.Pos++ // "}"
		return &compiler.SIf{Cond: condition, Then: body, Else: elseBody}
	}
	return &compiler.SIf{Cond: condition, Then: body}
}

func (p *Parser) parseComparison() compiler.Expr {
	term := p.parseTerm()
	fmt.Println(p.Tokens[p.Pos].Type)
	for p.Tokens[p.Pos].Type == "PUNCT" && (p.Tokens[p.Pos].Value == ">" || p.Tokens[p.Pos].Value == ">=" || p.Tokens[p.Pos].Value == "<" || p.Tokens[p.Pos].Value == "<=" || p.Tokens[p.Pos].Value == "==" || p.Tokens[p.Pos].Value == "!=") {
		op := p.Tokens[p.Pos].Value
		p.Pos++ // op
		rightTerm := p.parseTerm()
		switch op {
		case ">":
			term = compiler.EGt{Left: term, Right: rightTerm}
		case ">=":
			term = compiler.EEGt{Left: term, Right: rightTerm}
		case "<":
			term = compiler.ELt{Left: term, Right: rightTerm}
		case "<=":
			term = compiler.EELt{Left: term, Right: rightTerm}
		case "==":
			term = compiler.EEq{Left: term, Right: rightTerm}
		case "!=":
			term = compiler.ENEq{Left: term, Right: rightTerm}
		}
	}
	return term
}

func (p *Parser) parseExpression() compiler.Expr {
	term := p.parseComparison()
	for p.Tokens[p.Pos].Type == "PUNCT" && (p.Tokens[p.Pos].Value == "+" || p.Tokens[p.Pos].Value == "-") {
		op := p.Tokens[p.Pos].Value
		p.Pos++ // op
		rightTerm := p.parseComparison()
		if op == "+" {
			term = compiler.EAdd{Left: term, Right: rightTerm}
		} else {
			term = compiler.ESub{Left: term, Right: rightTerm}
		}
	}
	return term
}

func (p *Parser) parseTerm() compiler.Expr {
	factor := p.parseFactor()
	for p.Tokens[p.Pos].Type == "PUNCT" && (p.Tokens[p.Pos].Value == "*" || p.Tokens[p.Pos].Value == "/") {
		op := p.Tokens[p.Pos].Value
		p.Pos++ // op
		rightFactor := p.parseFactor()
		if op == "*" {
			factor = compiler.EMul{Left: factor, Right: rightFactor}
		} else {
			factor = compiler.EDiv{Left: factor, Right: rightFactor}
		}
	}
	return factor
}

func (p *Parser) parseFactor() compiler.Expr {
	switch p.Tokens[p.Pos].Type {
	case "NUMBER":
		if p.Tokens[p.Pos+1].Type == "IDENT" && isDurationUnit(p.Tokens[p.Pos+1].Value) {
			return p.parseDuration()
		}
		return p.parseNumber(false)
	case "STRING":
		return p.parseString()
	case "IDENT":
		if p.Tokens[p.Pos+1].Type == "PUNCT" && p.Tokens[p.Pos+1].Value == "(" {
			return p.parseNonVoidFunctionCall()
		}
		return p.parseIdentifier()
	case "PUNCT":
		if p.Tokens[p.Pos+1].Type == "NUMBER" {
			return p.parseNumber(true)
		} else {
			panic("Expected factor, found " + p.Tokens[p.Pos].Type)
		}
	default:
		panic("Expected factor, found " + p.Tokens[p.Pos].Type)
	}
}

func (p *Parser) parseNumber(symbol bool) compiler.Expr {
	value := p.Tokens[p.Pos].Value
	p.Pos++ // value
	if symbol {
		value += p.Tokens[p.Pos].Value
		p.Pos++
	}
	if strings.Contains(value, ".") {
		val, err := strconv.ParseFloat(value, 64)
		if err != nil {
			panic(err)
		}
		return compiler.EFloat{Value: val}
	} else {
		val, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			panic(err)
		}
		return compiler.EInt{Value: val}
	}
}

func (p *Parser) parseDuration() compiler.Expr {
	value, _ := strconv.ParseInt(p.Tokens[p.Pos].Value, 10, 64)
	p.Pos++ // value
	unit := p.Tokens[p.Pos].Value
	p.Pos++ // unit

	var duration time.Duration
	switch unit {
	case "ns":
		duration = time.Duration(value) * time.Nanosecond
	case "us":
		duration = time.Duration(value) * time.Microsecond
	case "ms":
		duration = time.Duration(value) * time.Millisecond
	case "s":
		duration = time.Duration(value) * time.Second
	case "m":
		duration = time.Duration(value) * time.Minute
	case "h":
		duration = time.Duration(value) * time.Hour
	default:
		panic("Unknown duration unit " + unit)
	}

	return compiler.EDuration{Value: duration}
}

func isDurationUnit(s string) bool {
	switch s {
	case "ns", "us", "ms", "s", "m", "h":
		return true
	default:
		return false
	}
}

func (p *Parser) parseString() compiler.Expr {
	value := p.Tokens[p.Pos].Value
	p.Pos++ // value
	value = strings.Trim(value, "\"")
	return compiler.EString{Value: value}
}

func (p *Parser) parseIdentifier() compiler.Expr {
	name := p.Tokens[p.Pos].Value
	p.Pos++ // value
	return compiler.EVar{Name: name}
}
