package analyzer

import (
	"fmt"

	"github.com/vyPal/CaffeineC/lib/parser"
)

type Program struct {
	Package string
	UseOOP  bool
}

func Analyze(ast *parser.Program) (*Program, error) {
	prog := &Program{
		Package: ast.Package,
		UseOOP:  false,
	}

	for _, stmt := range ast.Statements {
		if stmt.Comment != nil {
			if *stmt.Comment == "//cffc:force-oop" {
				prog.UseOOP = true
			}
		}
	}

	ctx := NewContext()
	AnalyzeStatements(ast.Statements, ctx, prog)
	return prog, nil
}

func AnalyzeStatements(stmts []*parser.Statement, ctx *Context, prog *Program) {
	for _, stmt := range stmts {
		if stmt.VariableDefinition != nil {
			ctx.Variables[stmt.VariableDefinition.Name] = Variable{
				Name:      stmt.VariableDefinition.Name,
				Type:      stringToType(stmt.VariableDefinition.Type),
				Constatnt: stmt.VariableDefinition.Constant,
			}
		} else if stmt.External != nil {
		} else if stmt.FunctionDefinition != nil {
		} else if stmt.ClassDefinition != nil {
		} else if stmt.Import != nil {
		} else if stmt.FromImport != nil {
		} else if stmt.FromImportMultiple != nil {
		} else if stmt.Export != nil {
		} else if stmt.Comment != nil {
		} else {
			if prog.UseOOP && ctx.topLevel {
				panic("Cannot have non-OOP code in OOP mode")
			} else {
				if stmt.Assignment != nil {
				} else if stmt.If != nil {
				} else if stmt.For != nil {
				} else if stmt.While != nil {
				} else if stmt.Return != nil {
				} else if stmt.FieldDefinition != nil {
				} else if stmt.Break != nil {
				} else if stmt.Continue != nil {
				} else if stmt.Expression != nil {
				} else {
					panic(fmt.Sprintf("Unknown statement: %v", stmt))
				}
			}
		}
	}
}
