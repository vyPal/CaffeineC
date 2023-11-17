package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
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
	module            *ir.Module
	symbolTable       map[string]constant.Constant
	tokens            []Token
	pos               int
	currentBlock      *ir.Block
	internalFunctions map[string]*ir.Func
}

func (p *Parser) Parse() {
	f := p.module.NewFunc("main", types.Void)
	p.currentBlock = f.NewBlock("main")
	p.registerInternalFunctions()

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
			} else if token.Value == "func" {
				p.parseFunctionDeclaration()
			} else if p.isFunctionName(p.tokens[p.pos].Value) {
				p.parseFunctionCall()
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
		case "int", "string", "float64", "duration":
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

func (p *Parser) parseFunctionDeclaration() {
	p.pos++ // "func"
	name := p.tokens[p.pos].Value
	p.pos++ // name
	p.pos++ // "("
	// Parse the parameters
	var params []*ir.Param
	for p.tokens[p.pos].Type != "PUNCT" || p.tokens[p.pos].Value != ")" {
		paramName := p.tokens[p.pos].Value
		p.pos++ // name
		p.pos++ // ":"
		paramType := p.tokens[p.pos].Value
		p.pos++ // type
		switch paramType {
		case "int":
			params = append(params, ir.NewParam(paramName, types.I64))
		case "string":
			params = append(params, ir.NewParam(paramName, types.NewPointer(types.I8)))
		case "float64":
			params = append(params, ir.NewParam(paramName, types.Double))
		case "duration":
			params = append(params, ir.NewParam(paramName, types.I64))
		default:
			panic(fmt.Sprintf("Unknown type %s", paramType))
		}
		if p.tokens[p.pos].Type == "PUNCT" && p.tokens[p.pos].Value == "," {
			p.pos++ // ","
		}
	}
	p.pos++ // ")"
	// Check if the function returns a value
	var returnType types.Type
	if p.tokens[p.pos].Type == "PUNCT" && p.tokens[p.pos].Value == ":" {
		p.pos++ // ":"
		switch p.tokens[p.pos].Value {
		case "int":
			returnType = types.I64
		case "string":
			returnType = types.NewPointer(types.I8)
		case "float64":
			returnType = types.Double
		case "duration":
			returnType = types.I64
		default:
			panic(fmt.Sprintf("Unknown type %s", p.tokens[p.pos].Value))
		}
	} else {
		returnType = types.Void
	}
	p.pos++ // type
	fmt.Printf("Declare function %s with return type %s\n", name, returnType)
	f := p.module.NewFunc(name, returnType, params...)
	prevBlock := p.currentBlock
	p.currentBlock = f.NewBlock("fn-" + name)
	for p.tokens[p.pos].Type != "PUNCT" || p.tokens[p.pos].Value != "}" {
		token := p.tokens[p.pos]
		switch token.Type {
		case "IDENT":
			if token.Value == "return" {
				p.pos++ // "return"
				value := p.parseExpression()
				fmt.Println("Return", value)
				p.currentBlock.NewRet(value)
				p.pos++ // ";"
			} else if token.Value == "print" {
				p.parsePrint()
			} else if token.Value == "sleep" {
				p.parseSleep()
			} else if token.Value == "func" {
				p.parseFunctionDeclaration()
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
	p.currentBlock = prevBlock
	p.symbolTable[name] = f
	p.pos++ // "}"
}

func (p *Parser) defineVariable(name string, val constant.Constant) {
	p.symbolTable[name] = val
}

func (p *Parser) registerInternalFunctions() {
	p.internalFunctions = make(map[string]*ir.Func)

	// Create a declaration for the printf function
	printf := p.module.NewFunc("printf", types.I32, ir.NewParam("", types.NewPointer(types.I8)))
	p.internalFunctions["printf"] = printf

	// Create a declaration for the sleep function
	sleep := p.module.NewFunc("sleep_ms", types.Void, ir.NewParam("", types.I64))
	p.internalFunctions["sleep"] = sleep
}

func (p *Parser) parsePrint() {
	p.pos++ // "print"
	val := p.parseExpression()

	// Create a declaration for the printf function
	printf := p.internalFunctions["printf"]

	// Create a call to printf
	var format *ir.Global
	for _, global := range p.module.Globals {
		if global.Name() == "format" {
			format = global
			break
		}
	}

	if format == nil {
		format = p.module.NewGlobalDef("format", constant.NewCharArrayFromString("%d\n"))
	}
	args := []value.Value{
		format,
		val,
	}
	p.currentBlock.NewCall(printf, args...)
	fmt.Println("Print", val)

	p.pos++ // ";"
}

func (p *Parser) parseSleep() {
	p.pos++ // "sleep"
	value := p.parseExpression()
	fmt.Println("Sleep", value)

	// Create a declaration for the sleep function
	sleep := p.internalFunctions["sleep"]

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
		if p.isFunctionName(p.tokens[p.pos].Value) {
			return p.parseNonVoidFunctionCall()
		}
		return p.parseIdentifier()
	default:
		panic("Expected factor, found " + p.tokens[p.pos].Type)
	}
}

func (p *Parser) isFunctionName(name string) bool {
	symbol, exists := p.symbolTable[name]
	if !exists {
		return false
	}
	_, isFunction := symbol.(*ir.Func)
	return isFunction
}

func (p *Parser) parseFunctionCall() {
	// Parse the function name
	name := p.tokens[p.pos].Value
	p.pos++

	// Get the function from the symbol table
	function := p.symbolTable[name].(*ir.Func)

	// Parse the argument list
	var args []value.Value
	p.pos++ // "("
	for p.tokens[p.pos].Type != "PUNCT" || p.tokens[p.pos].Value != ")" {
		args = append(args, p.parseExpression())
		if p.tokens[p.pos].Type == "PUNCT" && p.tokens[p.pos].Value == "," {
			p.pos++ // ","
		}
	}
	p.pos++ // ")"
	fmt.Println("Call", name, args)

	// Create a call instruction
	p.currentBlock.NewCall(function, args...)

	p.pos++ // ";"
}

func (p *Parser) parseNonVoidFunctionCall() constant.Constant {
	// Parse the function name
	name := p.tokens[p.pos].Value
	p.pos++

	// Get the function from the symbol table
	function := p.symbolTable[name].(*ir.Func)

	// Check if the function returns void
	if _, ok := function.Sig.RetType.(*types.VoidType); ok {
		panic("Function " + name + " returns void")
	}

	// Parse the argument list
	var args []value.Value
	p.pos++ // "("
	for p.tokens[p.pos].Type != "PUNCT" || p.tokens[p.pos].Value != ")" {
		args = append(args, p.parseExpression())
		if p.tokens[p.pos].Type == "PUNCT" && p.tokens[p.pos].Value == "," {
			p.pos++ // ","
		}
	}
	p.pos++ // ")"
	fmt.Println("Call", name, args)

	// Create a call instruction
	call := ir.NewCall(function, args...)

	// Add the call instruction to the current block
	p.currentBlock.NewCall(call)

	// Create a global variable to hold the result
	tmp := ir.NewGlobal("", function.Sig.RetType)

	// Store the result of the function call in the global variable
	p.currentBlock.NewStore(call, tmp)

	// Return the global variable as the result of the function call expression
	return tmp
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
	if len(os.Args) < 2 {
		fmt.Println("Usage: ./main <command> [<args>]")
		os.Exit(1)
	}

	var src []byte
	var filename string
	var no_cleanup *bool
	switch os.Args[1] {
	case "build":
		buildCmd := flag.NewFlagSet("build", flag.ExitOnError)
		no_cleanup = buildCmd.Bool("n", false, "Don't remove temporary files")

		// Parse the flags for the build command
		buildCmd.Parse(os.Args[2:])

		if buildCmd.NArg() < 1 {
			fmt.Println("Usage: ./main build [-n] <filename>")
			os.Exit(1)
		}

		filename = buildCmd.Arg(0)

		code, err := os.ReadFile(filename)
		if err != nil {
			fmt.Println("Error reading file:", err)
			os.Exit(1)
		}
		src = code

		// Use src and *noCleanup here...

	default:
		fmt.Println("Unknown command:", os.Args[1])
		fmt.Println("Usage: ./main <command> [<args>]")
		os.Exit(1)
	}

	l := Lexer{}
	l.s.Init(strings.NewReader(string(src)))
	l.s.Filename = filename
	tokens := l.Lex()

	mod := ir.NewModule()

	p := Parser{tokens: tokens, module: mod, symbolTable: make(map[string]constant.Constant)}
	p.Parse()

	err := os.WriteFile("output.ll", []byte(p.module.String()), 0644)
	if err != nil {
		log.Fatal(err)
	}

	// Compile the C code into an object file
	cmd := exec.Command("gcc", "-c", "./c_files/sleep.c", "-o", "sleep.o")
	err = cmd.Run()
	if err != nil {
		log.Fatal(err)
	}

	// Build the Go code
	cmd = exec.Command("llc", "-filetype=obj", "output.ll")
	err = cmd.Run()
	if err != nil {
		log.Fatal(err)
	}

	// Link everything together
	cmd = exec.Command("gcc", "output.o", "sleep.o", "-o", "output")
	err = cmd.Run()
	if err != nil {
		log.Fatal(err)
	}

	// Remove the temporary files
	fmt.Println(*no_cleanup)
	if !*no_cleanup {
		os.Remove("output.ll")
		os.Remove("output.o")
		os.Remove("sleep.o")
	}
}
