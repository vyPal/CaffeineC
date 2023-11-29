package compiler

import "github.com/vyPal/CaffeineC/lib/parser"

func (ctx *Context) compileStatement(s *parser.Statement) {
	if s.VariableDefinition != nil {

	} else if s.Assignment != nil {

	} else if s.FunctionDefinition != nil {

	} else if s.ClassDefinition != nil {

	} else if s.If != nil {

	} else if s.For != nil {

	} else if s.While != nil {

	} else if s.Return != nil {

	} else if s.Break != nil {

	} else if s.Continue != nil {

	} else if s.Expression != nil {

	} else if s.FieldDefinition != nil {

	} else {
		panic("Empty statement?")
	}
}
