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

func (ctx *Context) compileStatement(s *parser.Statement) error {
	if s.VariableDefinition != nil {
		ctx.compileVariableDefinition(s.VariableDefinition)
	} else if s.Assignment != nil {
		ctx.compileAssignment(s.Assignment)
	} else if s.FunctionDefinition != nil {
		ctx.compileFunctionDefinition(s.FunctionDefinition)
	} else if s.ClassDefinition != nil {
		ctx.compileClassDefinition(s.ClassDefinition)
	} else if s.If != nil {
		return ctx.compileIf(s.If)
	} else if s.For != nil {
		return ctx.compileFor(s.For)
	} else if s.While != nil {
		return ctx.compileWhile(s.While)
	} else if s.Return != nil {
		ctx.compileReturn(s.Return)
	} else if s.Break != nil {
		ctx.NewBr(ctx.fc.Leave)
	} else if s.Continue != nil {
		ctx.NewBr(ctx.fc.Continue)
	} else if s.Expression != nil {
		ctx.compileExpression(s.Expression)
	} else if s.FieldDefinition != nil {
		return cli.Exit(color.RedString("Error: Field definitions are not allowed outside of classes"), 1)
	} else if s.ExternalFunction != nil {
		ctx.compileExternalFunction(s.ExternalFunction)
	} else {
		panic("Empty statement?")
	}
	return nil
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
	alloc := ctx.NewAlloca(val.Type())
	ctx.NewStore(val, alloc)
	ctx.vars[v.Name] = alloc
	return v.Name, alloc.Type(), alloc, nil
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
		aptr, ok := variable.(*ir.InstAlloca)
		if !ok {
			ctx.vars[a.Left.Name] = val
		} else {
			ctx.NewStore(val, aptr)
		}
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
	block := fn.NewBlock("")
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

func (ctx *Context) compileClassDefinition(c *parser.ClassDefinition) (Name string, TypeDef *types.StructType, Methods []ir.Func) {
	classType := types.NewStruct()

	for _, s := range c.Body {
		if s.FieldDefinition != nil {
			classType.Fields = append(classType.Fields, stringToType(s.FieldDefinition.Type))
		} else if s.FunctionDefinition != nil {
			ctx.compileClassMethodDefinition(s.FunctionDefinition, c.Name, classType)
		}
	}

	ctx.Module.NewTypeDef(c.Name, classType)
	return c.Name, classType, nil
}

func (ctx *Context) compileClassMethodDefinition(f *parser.FunctionDefinition, cname string, ctype *types.StructType) {
	var params []*ir.Param
	for _, arg := range f.Parameters {
		params = append(params, ir.NewParam(arg.Name, stringToType(arg.Type)))
	}
	params = append(params, ir.NewParam("this", types.NewPointer(ctype)))

	fn := ctx.Module.NewFunc(f.Name, stringToType(f.ReturnType), params...)
	fn.Sig.Variadic = false
	fn.Sig.RetType = stringToType(f.ReturnType)
	block := fn.NewBlock("")
	nctx := NewContext(block, ctx.Compiler)
	for _, stmt := range f.Body {
		nctx.compileStatement(stmt)
	}
	if nctx.Term == nil {
		if stringToType(f.ReturnType).Equal(types.Void) {
			nctx.NewRet(nil)
		} else {
			cli.Exit(color.RedString("Error: Method `%s` of class `%s` does not return a value", f.Name, cname), 1)
		}
	}

	ctx.SymbolTable[cname+"."+f.Name] = fn
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
	// Compile the initializer
	if err := ctx.compileStatement(f.Initializer); err != nil {
		return err
	}

	// Create the loop and leave blocks
	loopB := ctx.Block.Parent.NewBlock("")
	leaveB := ctx.Block.Parent.NewBlock("")
	loopCtx := ctx.NewContext(loopB)

	// Compile the condition
	cond, err := ctx.compileExpression(f.Condition)
	if err != nil {
		return err
	}

	// Create a conditional branch to the loop or leave block based on the condition
	ctx.Block.NewCondBr(cond, loopB, leaveB)

	// Set the current block to the loop block
	ctx.Compiler.Context.Block = loopB

	// Compile the body of the loop
	for _, stmt := range f.Body {
		if err := loopCtx.compileStatement(stmt); err != nil {
			return err
		}
	}

	// Compile the increment expression
	if err := loopCtx.compileStatement(f.Increment); err != nil {
		return err
	}

	// Compile the condition again
	cond, err = loopCtx.compileExpression(f.Condition)
	if err != nil {
		return err
	}

	// Create a conditional branch to the loop or leave block based on the condition
	loopCtx.Block.NewCondBr(cond, loopB, leaveB)

	// Set the current block to the leave block
	ctx.Compiler.Context.Block = leaveB

	return nil
}

func (ctx *Context) compileWhile(w *parser.While) error {
	cond, err := ctx.compileExpression(w.Condition)
	if err != nil {
		return err
	}

	loopB := ctx.Block.Parent.NewBlock("")
	leaveB := ctx.Block.Parent.NewBlock("")
	loopCtx := ctx.NewContext(loopB)

	ctx.NewCondBr(cond, loopB, leaveB)
	loopCtx.fc.Leave = leaveB
	loopCtx.fc.Continue = loopB

	for _, stmt := range w.Body {
		loopCtx.compileStatement(stmt)
	}

	cond, err = loopCtx.compileExpression(w.Condition)
	if err != nil {
		return err
	}
	loopCtx.NewCondBr(cond, loopB, leaveB)
	ctx.Compiler.Context.Block = leaveB

	return nil
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
