package compiler

import (
	"strings"

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
		_, _, _, err := ctx.compileVariableDefinition(s.VariableDefinition)
		return err
	} else if s.Assignment != nil {
		_, _, err := ctx.compileAssignment(s.Assignment)
		return err
	} else if s.FunctionDefinition != nil {
		_, _, _, err := ctx.compileFunctionDefinition(s.FunctionDefinition)
		return err
	} else if s.ClassDefinition != nil {
		_, _, _, err := ctx.compileClassDefinition(s.ClassDefinition)
		return err
	} else if s.If != nil {
		return ctx.compileIf(s.If)
	} else if s.For != nil {
		return ctx.compileFor(s.For)
	} else if s.While != nil {
		return ctx.compileWhile(s.While)
	} else if s.Return != nil {
		return ctx.compileReturn(s.Return)
	} else if s.Break != nil {
		ctx.NewBr(ctx.fc.Leave)
	} else if s.Continue != nil {
		ctx.NewBr(ctx.fc.Continue)
	} else if s.Expression != nil {
		_, err := ctx.compileExpression(s.Expression)
		return err
	} else if s.FieldDefinition != nil {
		return posError(s.FieldDefinition.Pos, "Field definitions are not allowed outside of classes")
	} else if s.External != nil {
		ctx.compileExternalFunction(s.External)
	} else if s.Import != nil {
		//return ctx.Compiler.ImportAll(s.Import.Package, ctx)
	} else if s.FromImport != nil {
		/*
			symbols := map[string]string{strings.Trim(s.FromImport.Symbol, "\""): strings.Trim(s.FromImport.Symbol, "\"")}
			ctx.Compiler.ImportAs(s.FromImport.Package, symbols, ctx)
		*/
	} else if s.FromImportMultiple != nil {
		/*
			symbols := map[string]string{}
			for _, symbol := range s.FromImportMultiple.Symbols {
				if symbol.Alias == "" {
					symbol.Alias = symbol.Name
				}
				symbols[strings.Trim(symbol.Name, "\"")] = strings.Trim(symbol.Alias, "\"")
			}
			ctx.Compiler.ImportAs(s.FromImportMultiple.Package, symbols, ctx)
		*/
	} else if s.Export != nil {
		return ctx.compileStatement(s.Export)
	} else if s.Comment != nil {
		return nil
	} else {
		return posError(s.Pos, "Unknown statement")
	}
	return nil
}

func (ctx *Context) compileExternalFunction(v *parser.ExternalFunctionDefinition) {
	var retType types.Type
	if v.ReturnType == "" {
		retType = types.Void
	} else {
		retType = ctx.StringToType(v.ReturnType)
	}
	var args []*ir.Param
	for _, arg := range v.Parameters {
		args = append(args, ir.NewParam(arg.Name, ctx.StringToType(arg.Type)))
	}

	v.Name = strings.Trim(v.Name, "\"")

	fn := ctx.Module.NewFunc(v.Name, retType, args...)
	fn.Sig.Variadic = v.Variadic
}

func (ctx *Context) compileVariableDefinition(v *parser.VariableDefinition) (Name string, Type types.Type, Value value.Value, Err error) {
	// If there is no assignment, create an uninitialized variable
	valType := ctx.StringToType(v.Type)
	if v.Assignment == nil {
		alloc := ctx.NewAlloca(valType)
		ctx.NewStore(constant.NewZeroInitializer(valType), alloc)
		ctx.vars[v.Name] = &Variable{
			Name:  v.Name,
			Type:  valType,
			Value: alloc,
		}
		return v.Name, alloc.Type(), alloc, nil
	}

	if _, isPointer := valType.(*types.PointerType); isPointer && v.Assignment != nil {
		val, err := ctx.compileExpression(v.Assignment)
		if err != nil {
			return "", nil, nil, err
		}
		ctx.vars[v.Name] = &Variable{
			Name:  v.Name,
			Type:  valType,
			Value: val,
		}
		return v.Name, valType, val, nil
	}

	ctx.RequestedType = valType
	val, err := ctx.compileExpression(v.Assignment)
	if err != nil {
		return "", nil, nil, err
	}
	ctx.RequestedType = nil

	ptr, ok := val.(*ir.InstAlloca)
	if ok {
		ctx.vars[v.Name] = &Variable{
			Name:  v.Name,
			Type:  valType,
			Value: ptr,
		}
		return v.Name, ptr.Type(), ptr, nil
	}

	alloc := ctx.NewAlloca(val.Type())
	ctx.NewStore(val, alloc)
	ctx.vars[v.Name] = &Variable{
		Name:  v.Name,
		Type:  valType,
		Value: alloc,
	}
	return v.Name, alloc.Type(), alloc, nil
}

func (ctx *Context) compileAssignment(a *parser.Assignment) (Name string, Value value.Value, Err error) {
	// Compile the identifier to get the variable
	variable, vType, err := ctx.compileIdentifier(a.Left, false)
	if err != nil {
		return "", nil, err
	}

	if ptr, ok := variable.(*ir.InstGetElementPtr); ok {
		/*
			fmt.Println("This now")
			fmt.Println("vType: ", vType)
			fmt.Println("ElemType: ", ptr.ElemType)
		*/
		ctx.RequestedType = ptr.ElemType
	} else if ptr, ok := vType.(*types.PointerType); ok {
		ctx.RequestedType = ptr.ElemType
	} else {
		ctx.RequestedType = vType
	}
	val, err := ctx.compileExpression(a.Right)
	if err != nil {
		return "", nil, err
	}
	ctx.RequestedType = nil

	ptr, ok := variable.(*ir.InstGetElementPtr)
	if !ok {
		aptr, ok := variable.(*ir.InstAlloca)
		if !ok {
			ctx.vars[a.Left.Name] = &Variable{
				Name:  a.Left.Name,
				Type:  vType,
				Value: val,
			}
		} else {
			ctx.NewStore(val, aptr)
		}
	} else {
		ctx.NewStore(val, ptr)
	}

	return a.Left.Name, val, nil
}

func (ctx *Context) compileFunctionDefinition(f *parser.FunctionDefinition) (Name string, ReturnType types.Type, Args []*ir.Param, err error) {
	var params []*ir.Param
	for _, arg := range f.Parameters {
		params = append(params, ir.NewParam(arg.Name, ctx.StringToType(arg.Type)))
	}

	retType := ctx.StringToType(f.ReturnType)

	fn := ctx.Module.NewFunc(f.Name.Name, retType, params...)
	fn.Sig.Variadic = f.Variadic
	block := fn.NewBlock("")
	nctx := NewContext(block, ctx.Compiler)
	ctx.SymbolTable[f.Name.Name] = fn

	for _, stmt := range f.Body {
		err := nctx.compileStatement(stmt)
		if err != nil {
			return "", nil, []*ir.Param{}, err
		}
	}
	if nctx.Term == nil {
		if retType.Equal(types.Void) {
			nctx.NewRet(nil)
		} else {
			return "", nil, nil, posError(f.Pos, "Function `%s` does not return a value", f.Name.Name)
		}
	}

	return f.Name.Name, retType, params, nil
}

func (ctx *Context) compileClassDefinition(c *parser.ClassDefinition) (Name string, TypeDef *types.StructType, Methods []ir.Func, err error) {
	classType := types.NewStruct()
	classType.SetName(c.Name)
	ctx.structNames[classType] = c.Name
	ctx.Module.NewTypeDef(c.Name, classType)
	for _, s := range c.Body {
		if s.FieldDefinition != nil {
			classType.Fields = append(classType.Fields, ctx.StringToType(s.FieldDefinition.Type))
			ctx.Compiler.StructFields[c.Name] = append(ctx.Compiler.StructFields[c.Name], s.FieldDefinition)
		} else if s.FunctionDefinition != nil {
			err := ctx.compileClassMethodDefinition(s.FunctionDefinition, c.Name, classType)
			if err != nil {
				return "", nil, []ir.Func{}, err
			}
		}
	}

	return c.Name, classType, nil, nil
}

func (ctx *Context) compileClassMethodDefinition(f *parser.FunctionDefinition, cname string, ctype *types.StructType) error {
	var params []*ir.Param
	params = append(params, ir.NewParam("this", types.NewPointer(ctype)))
	for _, arg := range f.Parameters {
		params = append(params, ir.NewParam(arg.Name, ctx.StringToType(arg.Type)))
	}

	trimmed := strings.Trim(f.Name.String, "\"")
	ms := "." + f.Name.Name
	if f.Name.Op {
		ms = ".op." + trimmed
	} else if f.Name.Get {
		ms = ".get." + trimmed
	} else if f.Name.Set {
		ms = ".set." + trimmed
	}

	retType := ctx.StringToType(f.ReturnType)

	fn := ctx.Module.NewFunc(cname+ms, retType, params...)
	fn.Sig.Variadic = false
	block := fn.NewBlock("")
	nctx := NewContext(block, ctx.Compiler)
	ctx.SymbolTable[cname+ms] = fn
	for _, stmt := range f.Body {
		err := nctx.compileStatement(stmt)
		if err != nil {
			return err
		}
	}
	if nctx.Term == nil {
		if retType.Equal(types.Void) {
			nctx.NewRet(nil)
		} else {
			cli.Exit(color.RedString("Error: Method `%s` of class `%s` does not return a value", f.Name, cname), 1)
		}
	}

	return nil
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
		err := ctx.compileStatement(stmt)
		if err != nil {
			return err
		}
	}
	if thenBlock.Term == nil {
		thenBlock.NewBr(mergeBlock)
	}

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
			err := ctx.compileStatement(stmt)
			if err != nil {
				return err
			}
		}
		if thenBlock.Term == nil {
			thenBlock.NewBr(mergeBlock)
		}
		elseBlock = newElseBlock
	}

	// Compile the else part
	ctx.Block = elseBlock
	if i.Else != nil {
		for _, stmt := range i.Else {
			err := ctx.compileStatement(stmt)
			if err != nil {
				return err
			}
		}
	}
	if elseBlock.Term == nil {
		elseBlock.NewBr(mergeBlock)
	}

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
	ctx.Block = leaveB

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
		err := loopCtx.compileStatement(stmt)
		if err != nil {
			return err
		}
	}

	cond, err = loopCtx.compileExpression(w.Condition)
	if err != nil {
		return err
	}
	loopCtx.NewCondBr(cond, loopB, leaveB)
	ctx.Block = leaveB

	return nil
}

func (ctx *Context) compileReturn(r *parser.Return) error {
	if r.Expression != nil {
		ctx.RequestedType = ctx.Block.Parent.Sig.RetType
		val, err := ctx.compileExpression(r.Expression)
		if err != nil {
			return posError(r.Pos, "Error compiling return expression: %s", err.Error())
		}
		ctx.RequestedType = nil
		ctx.NewRet(val)
	} else {
		ctx.NewRet(nil)
	}
	return nil
}
