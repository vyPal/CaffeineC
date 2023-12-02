package compiler

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/llir/llvm/ir"
	"github.com/llir/llvm/ir/constant"
	"github.com/llir/llvm/ir/types"
	"github.com/llir/llvm/ir/value"
	"github.com/urfave/cli/v2"
	"github.com/vyPal/CaffeineC/lib/parser"
)

func (ctx *Context) compileStatement(s *parser.Statement) {
	if s.VariableDefinition != nil {
		ctx.compileVariableDefinition(s.VariableDefinition)
	} else if s.Assignment != nil {
		ctx.compileAssignment(s.Assignment)
	} else if s.FunctionDefinition != nil {
		ctx.compileFunctionDefinition(s.FunctionDefinition)
	} else if s.ClassDefinition != nil {
		panic("Not implemented") // TODO: Implement
	} else if s.If != nil {
		ctx.compileIf(s.If)
	} else if s.For != nil {
		ctx.compileFor(s.For)
	} else if s.While != nil {
		panic("Not implemented") // TODO: Implement
	} else if s.Return != nil {
		ctx.compileReturn(s.Return)
	} else if s.Break != nil {
		ctx.NewBr(ctx.fc.Leave)
	} else if s.Continue != nil {
		ctx.NewBr(ctx.fc.Continue)
	} else if s.Expression != nil {
		ctx.compileExpression(s.Expression)
	} else if s.FieldDefinition != nil {
		panic("Not implemented") // TODO: Implement
	} else if s.ExternalFunction != nil {
		ctx.compileExternalFunction(s.ExternalFunction)
	} else {
		panic("Empty statement?")
	}
}

func (ctx *Context) compileExternalFunction(v *parser.ExternalFunctionDefinition) {
	var retType types.Type
	if v.ReturnType == "" {
		retType = types.Void
	} else {
		retType = stringToType(v.ReturnType)
	}
	var args []*ir.Param
	for _, arg := range v.Parameters {
		args = append(args, ir.NewParam(arg.Name, stringToType(arg.Type)))
	}

	ctx.Module.NewFunc(v.Name, retType, args...)
}

func (ctx *Context) compileVariableDefinition(v *parser.VariableDefinition) (Name string, Type types.Type, Value value.Value, Err error) {
	val, err := ctx.compileExpression(v.Assignment)
	if err != nil {
		return "", nil, nil, err
	}
	ctx.vars[v.Name] = val
	return v.Name, val.Type(), val, nil
}

func (ctx *Context) compileAssignment(a *parser.Assignment) (Name string, Value value.Value, Err error) {
	val, err := ctx.compileExpression(a.Right)
	if err != nil {
		return "", nil, err
	}
	// Compile the identifier to get the variable
	variable, err := ctx.compileIdentifier(a.Left)
	if err != nil {
		return "", nil, err
	}

	ptr, ok := variable.(*ir.InstGetElementPtr)
	if !ok {
		ctx.vars[a.Left.Name] = val
	} else {
		ctx.Block.NewStore(val, ptr)
	}

	return a.Left.Name, val, nil
}

func (ctx *Context) compileFunctionDefinition(f *parser.FunctionDefinition) (Name string, ReturnType types.Type, Args []*ir.Param) {
	// Create a temporary context and block for analysis
	tmpBlock := ctx.Module.NewFunc("", types.Void)
	tmpCtx := ctx.NewContext(tmpBlock.NewBlock("tmp-entry"))

	var argsUsed []string
	for _, arg := range f.Parameters {
		argsUsed = append(argsUsed, arg.Name)
		tmpCtx.vars[arg.Name] = constant.NewInt(types.I1, 0)
		fmt.Println("Defined " + arg.Name)
	}
	for _, stmt := range f.Body {
		tmpCtx.compileStatement(stmt)
	}
	for name := range tmpCtx.usedVars {
		for _, arg := range argsUsed {
			if arg == name {
				continue
			}
		}
		ctx.usedVars[name] = true
		value := tmpCtx.lookupVariable(name)
		f.Parameters = append(f.Parameters, &parser.ArgumentDefinition{
			Name: name,
			Type: value.Type().Name(),
		})
	}
	if tmpCtx.Term == nil {
		if stringToType(f.ReturnType).Equal(types.Void) {
			tmpCtx.NewRet(nil)
		} else {
			cli.Exit(color.RedString("Error: Function `%s` does not return a value", f.Name), 1)
		}
	}

	// Remove the temporary function from the module
	funcs := []*ir.Func{}
	for _, f := range ctx.Module.Funcs {
		if f.Name() != tmpBlock.Name() {
			funcs = append(funcs, f)
		}
	}
	ctx.Module.Funcs = funcs

	var params []*ir.Param
	for _, arg := range f.Parameters {
		params = append(params, ir.NewParam(arg.Name, stringToType(arg.Type)))
	}

	fn := ctx.Module.NewFunc(f.Name, stringToType(f.ReturnType), params...)
	fn.Sig.Variadic = false
	fn.Sig.RetType = stringToType(f.ReturnType)
	block := fn.NewBlock("function-entry")
	nctx := NewContext(block, ctx.Compiler)
	for _, stmt := range f.Body {
		nctx.compileStatement(stmt)
	}
	if nctx.Term == nil {
		if stringToType(f.ReturnType).Equal(types.Void) {
			nctx.NewRet(nil)
		} else {
			cli.Exit(color.RedString("Error: Function `%s` does not return a value", f.Name), 1)
		}
	}

	ctx.SymbolTable[f.Name] = fn
	return f.Name, stringToType(f.ReturnType), params
}

func (ctx *Context) compileClassDefinition(c *parser.ClassDefinition) (Name string, TypeDef types.StructType, Methods []ir.Func) {
	panic("Not implemented")
}

func (ctx *Context) compileIf(i *parser.If) error {
	// Compile the condition
	cond, err := ctx.compileExpression(i.Condition)
	if err != nil {
		return err
	}

	// Create blocks for the then, else and merge parts
	thenBlock := ctx.Block.Parent.NewBlock("")
	elseBlock := ctx.Block.Parent.NewBlock("")
	mergeBlock := ctx.Block.Parent.NewBlock("")

	// Create the conditional branch
	ctx.Block.NewCondBr(cond, thenBlock, elseBlock)

	// Compile the then part
	ctx.Block = thenBlock
	for _, stmt := range i.Body {
		ctx.compileStatement(stmt)
	}
	thenBlock.NewBr(mergeBlock)

	// Compile the else if parts
	for _, elseif := range i.ElseIf {
		cond, err := ctx.compileExpression(elseif.Condition)
		if err != nil {
			return err
		}
		newElseBlock := ctx.Block.Parent.NewBlock("")
		ctx.Block = elseBlock
		ctx.Block.NewCondBr(cond, thenBlock, newElseBlock)
		ctx.Block = thenBlock
		for _, stmt := range elseif.Body {
			ctx.compileStatement(stmt)
		}
		thenBlock.NewBr(mergeBlock)
		elseBlock = newElseBlock
	}

	// Compile the else part
	ctx.Block = elseBlock
	if i.Else != nil {
		for _, stmt := range i.Else {
			ctx.compileStatement(stmt)
		}
	}
	elseBlock.NewBr(mergeBlock)

	// Continue with the merge block
	ctx.Block = mergeBlock
	return nil
}

func (ctx *Context) compileFor(f *parser.For) error {
	loopCtx := ctx.NewContext(ctx.Block.Parent.NewBlock(""))
	ctx.NewBr(loopCtx.Block)
	initName, _, initVal, err := loopCtx.compileVariableDefinition(f.Initializer)
	if err != nil {
		return err
	}
	firstAppear := loopCtx.NewPhi(ir.NewIncoming(initVal, ctx.Block))
	loopCtx.vars[initName] = firstAppear
	stepName, stepVal, err := loopCtx.compileAssignment(f.Increment)
	if err != nil {
		return err
	}
	firstAppear.Incs = append(firstAppear.Incs, ir.NewIncoming(stepVal, loopCtx.Block))
	loopCtx.vars[stepName] = stepVal
	leaveB := ctx.Block.Parent.NewBlock("")
	loopCtx.fc.Leave = leaveB
	for _, stmt := range f.Body {
		loopCtx.compileStatement(stmt)
	}
	expr, err := loopCtx.compileExpression(f.Condition)
	if err != nil {
		return err
	}
	loopCtx.NewCondBr(expr, loopCtx.Block, leaveB)
	ctx.Compiler.Context.Block = leaveB
	return nil
}

func (ctx *Context) compileWhile(w *parser.While) {

}

func (ctx *Context) compileReturn(r *parser.Return) {
	if r.Expression != nil {
		val, err := ctx.compileExpression(r.Expression)
		if err != nil {
			cli.Exit(color.RedString("Error: Unable to compile return expression"), 1)
		}
		ctx.NewRet(val)
	} else {
		ctx.NewRet(nil)
	}
}

func (ctx *Context) compileFieldDefinition(f *parser.FieldDefinition) {

}
