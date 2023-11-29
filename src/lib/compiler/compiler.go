package compiler

import (
	"github.com/fatih/color"
	"github.com/llir/llvm/ir"
	"github.com/llir/llvm/ir/constant"
	"github.com/llir/llvm/ir/types"
	"github.com/llir/llvm/ir/value"
	"github.com/vyPal/CaffeineC/lib/parser"
)

type Context struct {
	*ir.Block
	*Compiler
	parent     *Context
	vars       map[string]value.Value
	usedVars   map[string]bool
	leaveBlock *ir.Block
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
	for _, param := range c.Block.Parent.Params {
		if param.Name() == name {
			return param
		}
	}
	if v, ok := c.vars[name]; ok {
		return v
	} else if c.parent != nil {
		v := c.parent.lookupVariable(name)
		// Mark the variable as used in the parent context
		c.usedVars[name] = true
		return v
	} else {
		color.Red("Error: Unable to find a variable named: %s", name)
		panic("Variable not found")
	}
}

type Compiler struct {
	Module       *ir.Module
	SymbolTable  map[string]value.Value
	StructFields map[string][]parser.FieldDefinition
	Context      *Context
	AST          *parser.Program
}

func NewCompiler() *Compiler {
	return &Compiler{
		Module:       ir.NewModule(),
		SymbolTable:  make(map[string]value.Value),
		StructFields: make(map[string][]parser.FieldDefinition),
	}
}

func (c *Compiler) Compile(program *parser.Program) {
	c.AST = program
	fn := c.Module.NewFunc("main", types.I32)
	block := fn.NewBlock("entry")
	c.Context = NewContext(block, c)
	for _, s := range program.Statements {
		c.Context.compileStatement(s)
	}
	if c.Context.Term == nil {
		c.Context.NewRet(constant.NewInt(types.I32, 0))
	}
}
