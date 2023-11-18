package parser

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/llir/llvm/ir/constant"
	"github.com/llir/llvm/ir/enum"
	"github.com/llir/llvm/ir/types"
	"github.com/llir/llvm/ir/value"
)

func (p *Parser) parseStatement() {
	token := p.Tokens[p.Pos]
	switch token.Type {
	case "IDENT":
		if token.Value == "var" {
			p.parseVarDecl()
		} else if token.Value == "print" {
			p.parsePrint()
		} else if token.Value == "sleep" {
			p.parseSleep()
		} else if token.Value == "func" {
			p.parseFunctionDeclaration()
		} else if p.isFunctionName(p.Tokens[p.Pos].Value) {
			p.parseFunctionCall()
		} else {
			fmt.Println("[W]", token.Location, "Unexpected identifier:", token.Value)
			p.Pos++
		}
	default:
		fmt.Println("[W]", token.Location, "Unexpected token:", token.Value)
		p.Pos++
	}
}

func (p *Parser) parseExpression() value.Value {
	term := p.parseTerm()
	for p.Tokens[p.Pos].Type == "PUNCT" && (p.Tokens[p.Pos].Value == "+" || p.Tokens[p.Pos].Value == "-") {
		op := p.Tokens[p.Pos].Value
		p.Pos++ // op
		rightTerm := p.parseTerm()
		if op == "+" {
			if term.Type() == types.I64 && rightTerm.Type() == types.I64 {
				term = constant.NewAdd(term.(*constant.Int), rightTerm.(*constant.Int))
			} else {
				term = constant.NewFAdd(term.(constant.Constant), rightTerm.(constant.Constant))
			}
		} else {
			if term.Type() == types.I64 && rightTerm.Type() == types.I64 {
				term = constant.NewSub(term.(*constant.Int), rightTerm.(*constant.Int))
			} else {
				term = constant.NewFSub(term.(constant.Constant), rightTerm.(constant.Constant))
			}
		}
	}
	return term
}

func (p *Parser) parseTerm() value.Value {
	factor := p.parseFactor()
	for p.Tokens[p.Pos].Type == "PUNCT" && (p.Tokens[p.Pos].Value == "*" || p.Tokens[p.Pos].Value == "/") {
		op := p.Tokens[p.Pos].Value
		p.Pos++ // op
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

func (p *Parser) parseFactor() value.Value {
	switch p.Tokens[p.Pos].Type {
	case "NUMBER":
		if p.Tokens[p.Pos+1].Type == "IDENT" && isDurationUnit(p.Tokens[p.Pos+1].Value) {
			return p.parseDuration()
		}
		return p.parseNumber()
	case "STRING":
		return p.parseString()
	case "IDENT":
		if p.isFunctionName(p.Tokens[p.Pos].Value) {
			return p.parseNonVoidFunctionCall()
		}
		return p.parseIdentifier()
	default:
		panic("Expected factor, found " + p.Tokens[p.Pos].Type)
	}
}

func (p *Parser) parseNumber() value.Value {
	value := p.Tokens[p.Pos].Value
	p.Pos++ // value
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

func (p *Parser) parseDuration() value.Value {
	value, _ := strconv.ParseInt(p.Tokens[p.Pos].Value, 10, 64)
	p.Pos++ // value
	unit := p.Tokens[p.Pos].Value
	p.Pos++ // unit

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

func (p *Parser) parseString() value.Value {
	value := p.Tokens[p.Pos].Value
	p.Pos++               // value
	str := value + "\x00" // Add null terminator to the string
	global := p.Module.NewGlobalDef("", constant.NewCharArrayFromString(str))
	global.Linkage = enum.LinkagePrivate
	global.UnnamedAddr = enum.UnnamedAddrNone
	return global
}

func (p *Parser) parseIdentifier() value.Value {
	name := p.Tokens[p.Pos].Value
	p.Pos++ // value
	if val, ok := p.SymbolTable[name]; ok {
		return val
	}
	panic("Undefined identifier: " + name)
}
