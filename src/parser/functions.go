package parser

import (
	"fmt"
	"strings"

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
			lastPos := p.Pos
			for p.Tokens[p.Pos].Type != "PUNCT" || p.Tokens[p.Pos].Value != "}" {
				token := p.Tokens[p.Pos]
				switch token.Type {
				case "IDENT":
					if token.Value == "return" {
						p.Pos++ // "return"
						value := p.parseExpression()
						returnType = value.Type()
					} else {
						p.Pos++
					}
				default:
					p.Pos++
				}
			}
			p.Pos = lastPos
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
	if returnType != types.Void {
		p.Pos++ // "{"
	}
	fmt.Println("Start of function", name)
	for p.Tokens[p.Pos].Type != "PUNCT" || p.Tokens[p.Pos].Value != "}" {
		token := p.Tokens[p.Pos]
		switch token.Type {
		case "IDENT":
			if token.Value == "return" {
				p.Pos++ // "return"
				value := p.parseExpression()
				if !returnType.Equal(value.Type()) {
					panic(fmt.Sprintf("Function %s should return %s, but returned %s", name, returnType, value.Type()))
				}
				fmt.Println("Return", value)
				p.CurrentBlock.NewRet(value)
			} else if token.Value == "print" {
				p.parsePrint()
			} else if token.Value == "sleep" {
				p.parseSleep()
			} else if token.Value == "func" {
				p.parseFunctionDeclaration()
			} else {
				fmt.Println("[W]", token.Location, "Unexpected identifier:", token.Value)
				p.Pos++
			}
		default:
			fmt.Println("[W]", token.Location, "Unexpected token:", token.Value)
			p.Pos++
		}
	}
	if returnType == types.Void {
		p.CurrentBlock.NewRet(nil)
	}
	p.CurrentBlock = prevBlock
	fmt.Println("End of function", name)
	fmt.Println("Function", name, "has a return type of", returnType)
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
	fmt.Println("Call Print", val)

	// Create a declaration for the printf function
	printf := p.InternalFunctions["printf"]

	// Determine the format string based on the type of val
	var formatStr string
	switch t := val.Type().(type) {
	case *types.IntType:
		formatStr = "%d\n"
	case *types.FloatType:
		formatStr = "%f\n"
	case *types.PointerType:
		formatStr = "%s\n"
	case *types.ArrayType:
		if _, ok := t.ElemType.(*types.IntType); ok && t.ElemType.(*types.IntType).BitSize == 8 {
			formatStr = "%s\n"
		} else {
			panic("Unsupported type in print: " + val.Type().String())
		}
	default:
		panic("Unsupported type in print: " + val.Type().String())
	}

	// Create a unique name for the global definition
	formatName := "format_" + strings.ReplaceAll(formatStr, "\n", "")
	formatName = strings.ReplaceAll(formatName, "%", "")

	// Check for the existence of the global definition
	var format *ir.Global
	for _, global := range p.Module.Globals {
		if global.Name() == formatName {
			format = global
			break
		}
	}
	if format == nil {
		// If the global definition does not exist, create a new one
		format = p.Module.NewGlobalDef(formatName, constant.NewCharArrayFromString(formatStr))
	}

	if _, ok := val.Type().(*types.ArrayType); ok {
		zero := constant.NewInt(types.I32, 0)
		val = p.CurrentBlock.NewGetElementPtr(val.Type(), zero, zero)
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

func (p *Parser) parseNonVoidFunctionCall() value.Value {
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

	// Add the call instruction to the current block
	call := p.CurrentBlock.NewCall(function, args...)

	ret := p.CurrentBlock.NewRet(call)

	return ret.X
}
