package compiler

import (
	"strings"

	"github.com/fatih/color"
	"github.com/llir/llvm/ir"
	"github.com/llir/llvm/ir/constant"
	"github.com/llir/llvm/ir/enum"
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
		return ctx.compileAssignment(s.Assignment)
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
		return ctx.Compiler.ImportAll(s.Import.Package, ctx)
	} else if s.FromImport != nil {
		symbols := map[string]string{strings.Trim(s.FromImport.Symbol, "\""): strings.Trim(s.FromImport.Symbol, "\"")}
		ctx.Compiler.ImportAs(s.FromImport.Package, symbols, ctx)
	} else if s.FromImportMultiple != nil {
		symbols := map[string]string{}
		for _, symbol := range s.FromImportMultiple.Symbols {
			if symbol.Alias == "" {
				symbol.Alias = symbol.Name
			}
			symbols[strings.Trim(symbol.Name, "\"")] = strings.Trim(symbol.Alias, "\"")
		}
		ctx.Compiler.ImportAs(s.FromImportMultiple.Package, symbols, ctx)
	} else if s.Export != nil {
		return ctx.compileStatement(s.Export)
	} else if s.Comment != nil {
		return nil
	}
	return nil
}

func (ctx *Context) compileExternalFunction(v *parser.ExternalFunctionDefinition) {
	var retType types.Type
	if len(v.ReturnType) == 0 {
		retType = types.Void
	} else {
		retType = ctx.CFMultiTypeToLLType(v.ReturnType)
	}
	var args []*ir.Param
	for _, arg := range v.Parameters {
		args = append(args, ir.NewParam(arg.Name, ctx.CFTypeToLLType(arg.Type)))
	}

	v.Name = strings.Trim(v.Name, "\"")

	fn := ctx.Module.NewFunc(v.Name, retType, args...)
	fn.Sig.Variadic = v.Variadic
}

func (ctx *Context) compileVariableDefinition(v *parser.VariableDefinition) (Name string, Type types.Type, Value value.Value, Err error) {
	// If there is no assignment, create an uninitialized variable
	valType := ctx.CFTypeToLLType(v.Type)
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

func (ctx *Context) compileAssignment(a *parser.Assignment) (Err error) {
	type Ident struct {
		Value value.Value
		Type  types.Type
	}
	var idents = make([]Ident, len(a.Idents))

	for index, ident := range a.Idents {
		i, t, err := ctx.compileIdentifier(ident, false)
		if err != nil {
			return err
		}

		if a.Op != "=" && !isNumeric(t) {
			return posError(ident.Pos, "Numeric operator used on non-numeric identifier %s", ident.Name)
		}

		idents[index] = Ident{Value: i, Type: t}
	}

	ctx.RequestedType = idents[0].Type
	val, err := ctx.compileExpression(a.Right)
	if err != nil {
		return err
	}
	ctx.RequestedType = nil

	if a.Op != "=" {
		if !isNumeric(val.Type()) {
			return posError(a.Right.Pos, "Numeric operator used on non-numeric value")
		}

		for i, ident := range idents {
			_, isFloat := ident.Value.Type().(*types.FloatType)
			var v value.Value
			switch a.Op {
			case "+=":
				if isFloat {
					v = ctx.NewFAdd(ident.Value, val)
				} else {
					v = ctx.NewAdd(ident.Value, val)
				}
			case "-=":
				if isFloat {
					v = ctx.NewFSub(ident.Value, val)
				} else {
					v = ctx.NewSub(ident.Value, val)
				}
			case "*=":
				if isFloat {
					v = ctx.NewFMul(ident.Value, val)
				} else {
					v = ctx.NewMul(ident.Value, val)
				}
			case "/=":
				if isFloat {
					v = ctx.NewFDiv(ident.Value, val)
				} else {
					v = ctx.NewSDiv(ident.Value, val)
				}
			case "%=":
				if isFloat {
					return posError(a.Pos, "Modulus operator not allowed on float")
				}
				v = ctx.NewSRem(ident.Value, val)
			case "&=":
				v = ctx.NewAnd(ident.Value, val)
			case "|=":
				v = ctx.NewOr(ident.Value, val)
			case "^=":
				v = ctx.NewXor(ident.Value, val)
			case "<<=":
				v = ctx.NewShl(ident.Value, val)
			case ">>=":
				v = ctx.NewLShr(ident.Value, val)
			case ">>>=":
				v = ctx.NewAShr(ident.Value, val)
			case "??=":
				isNull := ctx.NewICmp(enum.IPredEQ, ident.Value, constant.NewNull(ident.Value.Type().(*types.PointerType)))
				v = ctx.NewSelect(isNull, val, ident.Value)
			}

			ptr, ok := ident.Value.(*ir.InstGetElementPtr)
			if !ok {
				aptr, ok := ident.Value.(*ir.InstAlloca)
				if !ok {
					ctx.vars[a.Idents[i].Name] = &Variable{
						Name:  a.Idents[i].Name,
						Type:  ident.Type,
						Value: v,
					}
				} else {
					ctx.NewStore(v, aptr)
				}
			} else {
				ctx.NewStore(v, ptr)
			}
		}
	} else {
		if len(idents) == 1 {
			ptr, ok := idents[0].Value.(*ir.InstGetElementPtr)
			if !ok {
				aptr, ok := idents[0].Value.(*ir.InstAlloca)
				if !ok {
					ctx.vars[a.Idents[0].Name] = &Variable{
						Name:  a.Idents[0].Name,
						Type:  idents[0].Type,
						Value: val,
					}
				} else {
					ctx.NewStore(val, aptr)
				}
			} else {
				ctx.NewStore(val, ptr)
			}
		} else {
			if _, ok := val.Type().(*types.StructType); !ok {
				return posError(a.Right.Pos, "Cannot assign non-struct value to multiple variables")
			}

			v := val.(*constant.Struct)
			if len(v.Fields) != len(idents) {
				return posError(a.Right.Pos, "Unable to unpack %d values into %d variables", len(v.Fields), len(idents))
			}

			for i, ident := range idents {
				ptr, ok := ident.Value.(*ir.InstGetElementPtr)
				if !ok {
					aptr, ok := ident.Value.(*ir.InstAlloca)
					if !ok {
						ctx.vars[a.Idents[i].Name] = &Variable{
							Name:  a.Idents[i].Name,
							Type:  ident.Type,
							Value: v.Fields[i],
						}
					} else {
						ctx.NewStore(v.Fields[i], aptr)
					}
				} else {
					ctx.NewStore(v.Fields[i], ptr)
				}
			}
		}
	}

	return nil
}

func (ctx *Context) compileFunctionDefinition(f *parser.FunctionDefinition) (Name string, ReturnType types.Type, Args []*ir.Param, err error) {
	var params []*ir.Param
	for _, arg := range f.Parameters {
		params = append(params, ir.NewParam(arg.Name, ctx.CFTypeToLLType(arg.Type)))
	}
	if f.Variadic != "" {
		params = append(params, ir.NewParam(f.Variadic, types.I8Ptr))
	}

	retType := ctx.CFMultiTypeToLLType(f.ReturnType)

	fn := ctx.Module.NewFunc(f.Name.Name, retType, params...)
	if f.Variadic != "" {
		fn.Sig.Variadic = true
	}
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
			classType.Fields = append(classType.Fields, ctx.CFTypeToLLType(s.FieldDefinition.Type))
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
		params = append(params, ir.NewParam(arg.Name, ctx.CFTypeToLLType(arg.Type)))
	}
	if f.Variadic != "" {
		params = append(params, ir.NewParam(f.Variadic, types.I8Ptr))
	}

	trimmed := strings.Trim(f.Name.Name, "\"")
	ms := "." + f.Name.Name
	if f.Name.Op {
		ms = ".op." + trimmed
	} else if f.Name.Get {
		ms = ".get." + trimmed
	} else if f.Name.Set {
		ms = ".set." + trimmed
	}

	retType := ctx.CFMultiTypeToLLType(f.ReturnType)

	fn := ctx.Module.NewFunc(cname+ms, retType, params...)
	if f.Variadic != "" {
		fn.Sig.Variadic = true
	}
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
	if len(r.Expressions) == 1 {
		ctx.RequestedType = ctx.Block.Parent.Sig.RetType
		val, err := ctx.compileExpression(r.Expressions[0])
		if err != nil {
			return posError(r.Pos, "Error compiling return expression: %s", err.Error())
		}
		ctx.RequestedType = nil
		ctx.NewRet(val)
	} else if len(r.Expressions) > 1 {
		if _, ok := ctx.Block.Parent.Sig.RetType.(*types.StructType); !ok {
			return posError(r.Pos, "Cannot return multiple values from a non-struct function")
		}

		var vals []constant.Constant
		for i, expr := range r.Expressions {
			ctx.RequestedType = ctx.Block.Parent.Sig.RetType.(*types.StructType).Fields[i]
			val, err := ctx.compileExpression(expr)
			ctx.RequestedType = nil
			if err != nil {
				return posError(r.Pos, "Error compiling return expression: %s", err.Error())
			}

			constVal, ok := val.(constant.Constant)
			if !ok {
				return posError(r.Pos, "Return expression did not evaluate to a constant")
			}

			vals = append(vals, constVal)
		}

		ctx.NewRet(constant.NewStruct(ctx.Block.Parent.Sig.RetType.(*types.StructType), vals...))
	} else {
		ctx.NewRet(nil)
	}
	return nil
}
