package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/scanner"
	"time"

	"github.com/llir/llvm/ir"
	"github.com/llir/llvm/ir/constant"
	"github.com/llir/llvm/ir/types"
	"github.com/llir/llvm/ir/value"
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
	fmt.Println(tokens)
	return tokens
}

type Parser struct {
	module       *ir.Module
	symbolTable  map[string]constant.Constant
	tokens       []Token
	pos          int
	currentBlock *ir.Block
}

func (p *Parser) Parse() {
	f := p.module.NewFunc("main", types.Void)
	p.currentBlock = f.NewBlock("")

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
	p.currentBlock.NewRet(nil)
}

func (p *Parser) parseVarDecl() {
	p.pos++ // "var"
	name := p.tokens[p.pos].Value
	p.pos++ // name
	p.pos++ // ":"
	typeName := p.tokens[p.pos].Value
	p.pos++ // type
	if p.tokens[p.pos].Type == "PUNCT" && p.tokens[p.pos].Value == "=" {
		p.pos++ // "="
		value := p.parseExpression()
		fmt.Printf("Declare variable %s of type %s with value %v\n", name, typeName, value)
		switch typeName {
		case "int", "string", "duration":
			p.module.NewGlobalDef(name, value)
		default:
			panic(fmt.Sprintf("Unknown type %s", typeName))
		}
		p.defineVariable(name, value)
	} else {
		fmt.Printf("Declare variable %s of type %s\n", name, typeName)
		switch typeName {
		case "int", "string", "duration":
			p.module.NewGlobalDef(name, constant.NewZeroInitializer(types.I64))
		default:
			panic(fmt.Sprintf("Unknown type %s", typeName))
		}
		p.defineVariable(name, constant.NewZeroInitializer(types.I64))
	}
	p.pos++ // ";"
}

func (p *Parser) defineVariable(name string, val constant.Constant) {
	p.symbolTable[name] = val
}

func (p *Parser) parsePrint() {
	p.pos++ // "print"
	val := p.parseExpression()

	// Create a declaration for the printf function
	printf := p.module.NewFunc("printf", types.I32, ir.NewParam("", types.NewPointer(types.I8)))

	// Create a call to printf
	format := constant.NewCharArrayFromString("%d\n")
	args := []value.Value{
		ir.NewGlobal(format.String(), types.NewArray(uint64(len(format.String())), types.I8)),
		val,
	}
	p.currentBlock.NewCall(printf, args...)

	p.pos++ // ";"
}

func (p *Parser) parseSleep() {
	p.pos++ // "sleep"
	value := p.parseExpression()

	// Create a declaration for the sleep function
	sleep := p.module.NewFunc("sleep", types.Void, ir.NewParam("", types.I64))

	// Create a call to sleep
	p.currentBlock.NewCall(sleep, value)

	p.pos++ // ";"
}

func (p *Parser) parseExpression() constant.Constant {
	term := p.parseTerm()
	for p.tokens[p.pos].Type == "PUNCT" && (p.tokens[p.pos].Value == "+" || p.tokens[p.pos].Value == "-") {
		op := p.tokens[p.pos].Value
		p.pos++ // op
		rightTerm := p.parseTerm()
		if op == "+" {
			if term.Type() == types.I64 && rightTerm.Type() == types.I64 {
				term = constant.NewAdd(term.(*constant.Int), rightTerm.(*constant.Int))
			} else {
				term = constant.NewFAdd(term, rightTerm)
			}
		} else {
			if term.Type() == types.I64 && rightTerm.Type() == types.I64 {
				term = constant.NewSub(term.(*constant.Int), rightTerm.(*constant.Int))
			} else {
				term = constant.NewFSub(term, rightTerm)
			}
		}
	}
	return term
}

func (p *Parser) parseTerm() constant.Constant {
	factor := p.parseFactor()
	for p.tokens[p.pos].Type == "PUNCT" && (p.tokens[p.pos].Value == "*" || p.tokens[p.pos].Value == "/") {
		op := p.tokens[p.pos].Value
		p.pos++ // op
		rightFactor := p.parseFactor()
		if op == "*" {
			if factor.Type() == types.I64 && rightFactor.Type() == types.I64 {
				factor = constant.NewMul(factor.(*constant.Int), rightFactor.(*constant.Int))
			} else {
				factor = constant.NewFMul(factor.(*constant.Float), rightFactor.(*constant.Float))
			}
		} else {
			if factor.Type() == types.I64 && rightFactor.Type() == types.I64 {
				factor = constant.NewSDiv(factor.(*constant.Int), rightFactor.(*constant.Int))
			} else {
				factor = constant.NewFDiv(factor.(*constant.Float), rightFactor.(*constant.Float))
			}
		}
	}
	return factor
}

func (p *Parser) parseFactor() constant.Constant {
	switch p.tokens[p.pos].Type {
	case "NUMBER":
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

func (p *Parser) parseNumber() constant.Constant {
	value := p.tokens[p.pos].Value
	p.pos++ // value
	if strings.Contains(value, ".") {
		val, err := strconv.ParseFloat(value, 64)
		if err != nil {
			panic(err)
		}
		if strings.Contains(value, "e") {
			return constant.NewFloat(types.Double, val)
		} else {
			return constant.NewFloat(types.Float, val)
		}
	} else {
		val, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			panic(err)
		}
		return constant.NewInt(types.I64, val)
	}
}

func (p *Parser) parseDuration() constant.Constant {
	value, _ := strconv.ParseInt(p.tokens[p.pos].Value, 10, 64)
	p.pos++ // value
	unit := p.tokens[p.pos].Value
	p.pos++ // unit

	// Convert the value to nanoseconds
	switch unit {
	case "ns":
		// value is already in nanoseconds
	case "us":
		value *= int64(time.Microsecond)
	case "ms":
		value *= int64(time.Millisecond)
	case "s":
		value *= int64(time.Second)
	case "m":
		value *= int64(time.Minute)
	case "h":
		value *= int64(time.Hour)
	default:
		panic("Unknown duration unit: " + unit)
	}

	return constant.NewInt(types.I64, value)
}

func isDurationUnit(s string) bool {
	switch s {
	case "ns", "us", "ms", "s", "m", "h":
		return true
	default:
		return false
	}
}

func (p *Parser) parseString() constant.Constant {
	value := p.tokens[p.pos].Value
	p.pos++ // value
	return constant.NewCharArray([]byte(value))
}

func (p *Parser) parseIdentifier() constant.Constant {
	name := p.tokens[p.pos].Value
	p.pos++ // value
	if val, ok := p.symbolTable[name]; ok {
		return val
	}
	panic("Undefined identifier: " + name)
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

	mod := ir.NewModule()

	p := Parser{tokens: tokens, module: mod, symbolTable: make(map[string]constant.Constant)}
	p.Parse()

	fmt.Println("\n---\nModule:\n---")
	fmt.Println(mod)
}
