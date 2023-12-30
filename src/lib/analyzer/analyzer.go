package analyzer

import (
	"github.com/fatih/color"
	"github.com/urfave/cli/v2"
	"github.com/vyPal/CaffeineC/lib/parser"
)

func Analyze(ast *parser.Program) {
	for _, v := range ast.Statements {
		if v.VariableDefinition != nil {
			// analyzeVariableDefinition(v.VariableDefinition)
		} else if v.Assignment != nil {
			// analyzeAssignment(v.Assignment)
		} else if v.FunctionDefinition != nil {
			// analyzeFunctionDefinition(v.FunctionDefinition)
		} else if v.ClassDefinition != nil {
			// analyzeClassDefinition(v.ClassDefinition)
		} else if v.If != nil {
			// analyzeIf(v.If)
		} else if v.For != nil {
			// analyzeFor(v.For)
		} else if v.While != nil {
			// analyzeWhile(v.While)
		} else if v.Return != nil {
			// analyzeReturn(v.Return)
		} else if v.Break != nil {
			// analyzeBreak(v.Break)
		} else if v.Continue != nil {
			// analyzeContinue(v.Continue)
		} else if v.Expression != nil {
			// analyzeExpression(v.Expression)
		} else if v.FieldDefinition != nil {
			// analyzeFieldDefinition(v.FieldDefinition)
		} else if v.External != nil {
			if v.External.Function != nil {
				analyzeExternalFunction(v.External.Function)
			}
		} else {
			cli.Exit(color.RedString("Error: Empty statement?"), 1)
		}
	}
}

func analyzeExternalFunction(v *parser.ExternalFunctionDefinition) {
	if v.ReturnType == "" {
		color.Yellow("Warning: External function %s has no return type, 'void' will be used", v.Name)
	}
}
