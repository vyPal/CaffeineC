package main

import (
	"fmt"
	"os"
	"strings"
	"text/scanner"
)

type Token struct {
	Type     string
	Value    string
	Location scanner.Position
}

type Lexer struct {
	s scanner.Scanner
}

func (l *Lexer) Lex() []Token {
	var tokens []Token
	for tok := l.s.Scan(); tok != scanner.EOF; tok = l.s.Scan() {
		switch tok {
		case scanner.Ident:
			tokens = append(tokens, Token{"IDENT", l.s.TokenText(), l.s.Pos()})
		case scanner.Int, scanner.Float:
			tokens = append(tokens, Token{"NUMBER", l.s.TokenText(), l.s.Pos()})
		case scanner.String:
			tokens = append(tokens, Token{"STRING", l.s.TokenText(), l.s.Pos()})
		default:
			tokens = append(tokens, Token{"PUNCT", l.s.TokenText(), l.s.Pos()})
		}
	}
	return tokens
}

type Parser struct {
	tokens []Token
	pos    int
}

func (p *Parser) Parse() {
	for p.pos < len(p.tokens) {
		token := p.tokens[p.pos]
		switch token.Type {
		case "IDENT":
			if token.Value == "var" {
				p.parseVarDecl()
			} else if token.Value == "print" {
				p.parsePrint()
			} else if token.Value == "sleep" {
				p.parseSleep()
			} else {
				fmt.Println("[W]", token.Location, "Unexpected identifier:", token.Value)
				p.pos++
			}
		default:
			fmt.Println("[W]", token.Location, "Unexpected token:", token.Value)
			p.pos++
		}
	}
}

func (p *Parser) parseVarDecl() {
	p.pos++ // "var"
	name := p.tokens[p.pos].Value
	p.pos++ // name
	p.pos++ // ":"
	p.pos++ // type
	if p.tokens[p.pos].Type == "PUNCT" && p.tokens[p.pos].Value == "=" {
		p.pos++ // "="
		value := p.parseExpression()
		fmt.Printf("Declare variable %s with value %v\n", name, value)
	}
	p.pos++ // ";"
}

func (p *Parser) parsePrint() {
	p.pos++ // "print"
	value := p.parseExpression()
	fmt.Printf("Print %v\n", value)
	p.pos++ // ";"
}

func (p *Parser) parseSleep() {
	p.pos++ // "sleep"
	value := p.parseExpression()
	fmt.Printf("Sleep %v\n", value)
	p.pos++ // ";"
}

func (p *Parser) parseExpression() string {
	term := p.parseTerm()
	for p.tokens[p.pos].Type == "PUNCT" && (p.tokens[p.pos].Value == "+" || p.tokens[p.pos].Value == "-") {
		op := p.tokens[p.pos].Value
		p.pos++ // op
		rightTerm := p.parseTerm()
		term = fmt.Sprintf("(%s %s %s)", term, op, rightTerm)
	}
	return term
}

func (p *Parser) parseTerm() string {
	factor := p.parseFactor()
	for p.tokens[p.pos].Type == "PUNCT" && (p.tokens[p.pos].Value == "*" || p.tokens[p.pos].Value == "/") {
		op := p.tokens[p.pos].Value
		p.pos++ // op
		rightFactor := p.parseFactor()
		factor = fmt.Sprintf("(%s %s %s)", factor, op, rightFactor)
	}
	return factor
}

func (p *Parser) parseFactor() string {
	switch p.tokens[p.pos].Type {
	case "NUMBER":
		// If the next token is an identifier and it's a duration unit, parse a duration
		if p.tokens[p.pos+1].Type == "IDENT" && isDurationUnit(p.tokens[p.pos+1].Value) {
			return p.parseDuration()
		}
		return p.parseNumber()
	case "STRING":
		return p.parseString()
	case "IDENT":
		return p.parseIdentifier()
	default:
		panic("Expected factor, found " + p.tokens[p.pos].Type)
	}
}

func (p *Parser) parseNumber() string {
	value := p.tokens[p.pos].Value
	p.pos++ // value
	return value
}

func (p *Parser) parseDuration() string {
	value := p.tokens[p.pos].Value
	p.pos++ // value
	unit := p.tokens[p.pos].Value
	p.pos++ // unit
	return value + unit
}

func isDurationUnit(s string) bool {
	switch s {
	case "ns", "us", "ms", "s", "m", "h":
		return true
	default:
		return false
	}
}

func (p *Parser) parseString() string {
	value := p.tokens[p.pos].Value
	p.pos++ // value
	return value
}

func (p *Parser) parseIdentifier() string {
	value := p.tokens[p.pos].Value
	p.pos++ // value
	return value
}

func main() {
	const filename = "example.caffc"
	src, err := os.ReadFile(filename)
	if err != nil {
		fmt.Println("Error reading file:", err)
		os.Exit(1)
	}

	l := Lexer{}
	l.s.Init(strings.NewReader(string(src)))
	l.s.Filename = filename
	tokens := l.Lex()

	p := Parser{tokens: tokens}
	p.Parse()
}
