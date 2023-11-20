package compiler

import (
	"fmt"

	"github.com/llir/llvm/ir"
	"github.com/llir/llvm/ir/constant"
	"github.com/llir/llvm/ir/types"
	"github.com/llir/llvm/ir/value"
)

type Context struct {
	*ir.Block
	parent   *Context
	vars     map[string]value.Value
	usedVars map[string]bool
	*Compiler
}

func NewContext(b *ir.Block, comp *Compiler) *Context {
	return &Context{
		Block:    b,
		Compiler: comp,
		parent:   nil,
		vars:     make(map[string]value.Value),
		usedVars: make(map[string]bool),
	}
}

func (c *Context) NewContext(b *ir.Block) *Context {
	ctx := NewContext(b, c.Compiler)
	ctx.parent = c
	return ctx
}

func (c Context) lookupVariable(name string) value.Value {
	if v, ok := c.vars[name]; ok {
		return v
	} else if c.parent != nil {
		v := c.parent.lookupVariable(name)
		// Mark the variable as used in the parent context
		c.usedVars[name] = true
		return v
	} else {
		if c.Compiler.VarsCanBeNumbers {
			return nil
		}
		fmt.Printf("variable: `%s`\n", name)
		panic("no such variable")
	}
}

type Compiler struct {
	Module           *ir.Module
	SymbolTable      map[string]value.Value
	Context          *Context
	AST              []Stmt
	VarsCanBeNumbers bool
}

func (c *Compiler) Compile() {
	c.defineBuiltinFunctions()
	funcMain := c.Module.NewFunc("main", types.I32)
	funcMain.Sig.Variadic = true
	funcMain.Sig.RetType = types.I32
	block := funcMain.NewBlock("entry")
	c.Context = NewContext(block, c)
	for _, stmt := range c.AST {
		fmt.Printf("%T: %v\n", stmt, stmt)
		c.Context.compileStmt(stmt)
	}
	if c.Context.Block.Term == nil {
		c.Context.Block.NewRet(constant.NewInt(types.I32, 0))
	}
	fmt.Println("Symbol table:")
	for k, v := range c.SymbolTable {
		fmt.Printf("%s: %v\n", k, v)
	}

	fmt.Println("Vars:")
	for k, v := range c.Context.vars {
		fmt.Printf("%s: %v\n", k, v)
	}
}

func (c *Compiler) defineBuiltinFunctions() {
	printFunc := c.Module.NewFunc("printf", types.I32, ir.NewParam("", types.NewPointer(types.I8)))
	printFunc.Sig.Variadic = true
	printFunc.Sig.RetType = types.I32

	sleepFunc := c.Module.NewFunc("sleep_ns", types.Void, ir.NewParam("", types.I64))
	sleepFunc.Sig.Variadic = true
	sleepFunc.Sig.RetType = types.Void

	c.SymbolTable["printf"] = printFunc
	c.SymbolTable["sleep_ns"] = sleepFunc
}
