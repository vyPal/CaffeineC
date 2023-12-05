package compiler

import (
	"github.com/fatih/color"
	"github.com/llir/llvm/ir"
	"github.com/llir/llvm/ir/constant"
	"github.com/llir/llvm/ir/types"
	"github.com/llir/llvm/ir/value"
	"github.com/urfave/cli/v2"
	"github.com/vyPal/CaffeineC/lib/parser"
)

type Context struct {
	*ir.Block
	*Compiler
	parent   *Context
	vars     map[string]value.Value
	usedVars map[string]bool
	fc       *FlowControl
}

type FlowControl struct {
	Leave    *ir.Block
	Continue *ir.Block
}

func NewContext(b *ir.Block, comp *Compiler) *Context {
	return &Context{
		Block:    b,
		Compiler: comp,
		parent:   nil,
		vars:     make(map[string]value.Value),
		usedVars: make(map[string]bool),
		fc:       &FlowControl{},
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
		cli.Exit(color.RedString("Error: Unable to find a variable named: %s", name), 1)
		return nil
	}
}

func (c *Context) lookupFunction(name string) (*ir.Func, bool) {
	for _, f := range c.Module.Funcs {
		if f.Name() == name {
			return f, true
		}
	}

	return nil, false
}

func (c Context) lookupClass(name string) (types.Type, bool) {
	for _, s := range c.Module.TypeDefs {
		if s.Name() == name {
			return s, true
		}
	}
	return nil, false
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
	block := fn.NewBlock("")
	c.Context = NewContext(block, c)
	for _, s := range program.Statements {
		c.Context.compileStatement(s)
	}
	if c.Context.Term == nil {
		c.Context.NewRet(constant.NewInt(types.I32, 0))
	}
}
