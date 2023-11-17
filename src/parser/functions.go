package parser

import (
	"fmt"

	"github.com/llir/llvm/ir"
	"github.com/llir/llvm/ir/constant"
	"github.com/llir/llvm/ir/types"
	"github.com/llir/llvm/ir/value"
)

func (p *Parser) parseFunctionDeclaration() {
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
			params = append(params, ir.NewParam(paramName, types.NewPointer(types.I8)))
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
			returnType = types.NewPointer(types.I8)
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
	f := p.Module.NewFunc(name, returnType, params...)
	prevBlock := p.CurrentBlock
	p.CurrentBlock = f.NewBlock("fn-" + name)
	for p.Tokens[p.Pos].Type != "PUNCT" || p.Tokens[p.Pos].Value != "}" {
		p.parseStatement()
	}
	p.CurrentBlock.NewRet(nil)
	p.CurrentBlock = prevBlock
	p.SymbolTable[name] = f
	p.Pos++ // "}"
}

func (p *Parser) registerInternalFunctions() {
	p.InternalFunctions = make(map[string]*ir.Func)

	// Create a declaration for the printf function
	printf := p.Module.NewFunc("printf", types.I32, ir.NewParam("", types.NewPointer(types.I8)))
	p.InternalFunctions["printf"] = printf

	// Create a declaration for the sleep function
	sleep := p.Module.NewFunc("sleep_ms", types.Void, ir.NewParam("", types.I64))
	p.InternalFunctions["sleep"] = sleep
}

func (p *Parser) parsePrint() {
	p.Pos++ // "print"
	val := p.parseExpression()

	// Create a declaration for the printf function
	printf := p.InternalFunctions["printf"]

	// Create a call to printf
	var format *ir.Global
	for _, global := range p.Module.Globals {
		if global.Name() == "format" {
			format = global
			break
		}
	}

	if format == nil {
		format = p.Module.NewGlobalDef("format", constant.NewCharArrayFromString("%d\n"))
	}
	args := []value.Value{
		format,
		val,
	}
	p.CurrentBlock.NewCall(printf, args...)
	fmt.Println("Print", val)

	p.Pos++ // ";"
}

func (p *Parser) parseSleep() {
	p.Pos++ // "sleep"
	value := p.parseExpression()
	fmt.Println("Sleep", value)

	// Create a declaration for the sleep function
	sleep := p.InternalFunctions["sleep"]

	// Create a call to sleep
	p.CurrentBlock.NewCall(sleep, value)

	p.Pos++ // ";"
}

func (p *Parser) isFunctionName(name string) bool {
	symbol, exists := p.SymbolTable[name]
	if !exists {
		return false
	}
	_, isFunction := symbol.(*ir.Func)
	return isFunction
}

func (p *Parser) parseFunctionCall() {
	// Parse the function name
	name := p.Tokens[p.Pos].Value
	p.Pos++

	// Get the function from the symbol table
	function := p.SymbolTable[name].(*ir.Func)

	// Parse the argument list
	var args []value.Value
	p.Pos++ // "("
	for p.Tokens[p.Pos].Type != "PUNCT" || p.Tokens[p.Pos].Value != ")" {
		args = append(args, p.parseExpression())
		if p.Tokens[p.Pos].Type == "PUNCT" && p.Tokens[p.Pos].Value == "," {
			p.Pos++ // ","
		}
	}
	p.Pos++ // ")"
	fmt.Println("Call", name, args)

	// Create a call instruction
	p.CurrentBlock.NewCall(function, args...)

	p.Pos++ // ";"
}

func (p *Parser) parseNonVoidFunctionCall() constant.Constant {
	// Parse the function name
	name := p.Tokens[p.Pos].Value
	p.Pos++

	// Get the function from the symbol table
	function := p.SymbolTable[name].(*ir.Func)

	// Check if the function returns void
	if _, ok := function.Sig.RetType.(*types.VoidType); ok {
		panic("Function " + name + " returns void")
	}

	// Parse the argument list
	var args []value.Value
	p.Pos++ // "("
	for p.Tokens[p.Pos].Type != "PUNCT" || p.Tokens[p.Pos].Value != ")" {
		args = append(args, p.parseExpression())
		if p.Tokens[p.Pos].Type == "PUNCT" && p.Tokens[p.Pos].Value == "," {
			p.Pos++ // ","
		}
	}
	p.Pos++ // ")"
	fmt.Println("Call", name, args)

	// Create a call instruction
	call := ir.NewCall(function, args...)

	// Add the call instruction to the current block
	p.CurrentBlock.NewCall(call)

	// Create a global variable to hold the result
	tmp := ir.NewGlobal("", function.Sig.RetType)

	// Store the result of the function call in the global variable
	p.CurrentBlock.NewStore(call, tmp)

	// Return the global variable as the result of the function call expression
	return tmp
}
