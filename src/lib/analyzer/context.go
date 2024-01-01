package analyzer

import "github.com/vyPal/CaffeineC/lib/parser"

type Context struct {
	topLevel  bool
	Parent    *Context
	Variables map[string]Variable
	Functions map[string]Function
	Types     map[string]TypeDef
	Classes   map[string]Class
}

type Variable struct {
	Name      string
	Type      Type
	Constatnt bool
}

type Function struct {
	Name       string
	Parameters []Variable
	ReturnType Type
	Body       []parser.Statement
}

type TypeDef struct {
	Name   string
	Fields []Variable
	Type   Type
}

type Class struct {
	Name       string
	Methods    []Function
	Properties []Variable
}

func NewContext() *Context {
	return &Context{
		topLevel:  true,
		Parent:    nil,
		Variables: make(map[string]Variable),
		Functions: make(map[string]Function),
		Types:     make(map[string]TypeDef),
		Classes:   make(map[string]Class),
	}
}

func (c *Context) NewContext() *Context {
	return &Context{
		topLevel:  false,
		Parent:    c,
		Variables: make(map[string]Variable),
		Functions: make(map[string]Function),
		Types:     make(map[string]TypeDef),
		Classes:   make(map[string]Class),
	}
}

func (c *Context) LookupVariable(name string) (Variable, bool) {
	if v, ok := c.Variables[name]; ok {
		return v, true
	} else if c.Parent != nil {
		return c.Parent.LookupVariable(name)
	} else {
		return Variable{}, false
	}
}

func (c *Context) LookupFunction(name string) (Function, bool) {
	if v, ok := c.Functions[name]; ok {
		return v, true
	} else if c.Parent != nil {
		return c.Parent.LookupFunction(name)
	} else {
		return Function{}, false
	}
}

func (c *Context) LookupTypeDef(name string) (TypeDef, bool) {
	if v, ok := c.Types[name]; ok {
		return v, true
	} else if c.Parent != nil {
		return c.Parent.LookupTypeDef(name)
	} else {
		return TypeDef{}, false
	}
}
