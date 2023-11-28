package compiler

import (
	"fmt"

	"github.com/llir/llvm/ir"
	"github.com/llir/llvm/ir/types"
	"github.com/llir/llvm/ir/value"
)

type Class struct {
	Stmt
	Name        string
	Constructor Method
	Fields      []Field
	Methods     []Method
}

type Field struct {
	Name    string
	Type    types.Type
	Private bool
	Index   int
}

type Method struct {
	Name       string
	Params     []*CParam
	ReturnType *CType
	Body       []Stmt
	Private    bool
}

func (ctx *Context) compileClassDeclaration(c *Class) {
	classType := types.NewStruct()
	for _, field := range c.Fields {
		classType.Fields = append(classType.Fields, field.Type)
	}
	ctx.Compiler.StructFields[c.Name] = c.Fields
	fmt.Println(c.Constructor.Body[0])
	ctx.Module.NewTypeDef(c.Name, classType)
	if c.Constructor.Name != "" {
		fmt.Println("Declaring constructor " + c.Constructor.Name)
		constructor := ctx.compileMethodDeclaration(c.Constructor, c)
		// Add constructor to the symbol table
		ctx.Compiler.SymbolTable[c.Name] = constructor
	}
	for _, m := range c.Methods {
		fmt.Println("Declaring method " + m.Name)
		ctx.Compiler.SymbolTable[c.Name+"."+m.Name] = ctx.compileMethodDeclaration(m, c)
	}
}

func (ctx *Context) compileMethodDeclaration(m Method, c *Class) *ir.Func {
	params := m.Params
	var typeDef types.Type
	for _, t := range ctx.Module.TypeDefs {
		if t.Name() == c.Name {
			typeDef = t
			break
		}
	}

	if typeDef == nil {
		panic("type not found")
	}
	params = append(params, &CParam{Name: "this", Typ: CType{Typ: types.NewPointer(typeDef)}})
	f := ctx.Module.NewFunc(m.Name, ctx.toType(m.ReturnType))
	f.Params = ctx.toParams(params)
	block := f.NewBlock("entry")
	classctx := ctx.NewContext(block)
	for _, stmt := range m.Body {
		classctx.compileStmt(stmt)
	}
	if classctx.Term == nil {
		if ctx.toType(m.ReturnType).Equal(types.Void) {
			classctx.NewRet(nil)
		} else {
			panic(fmt.Errorf("function `%s` does not return a value", m.Name))
		}
	}

	return f
}

func (ctx *Context) compileClassMethod(c SClassMethod) {
	// Get the class instance
	instance := ctx.lookupVariable(c.InstanceName)
	fmt.Println("Instance:", instance)

	var classType types.Type
	for _, t := range ctx.Module.TypeDefs {
		if t.Equal(instance.Type().(*types.PointerType).ElemType) {
			classType = t
			break
		}
	}
	if classType == nil {
		panic(fmt.Errorf("type `%s` not found", instance.Type().String()))
	}
	// Get the method
	method := ctx.Compiler.SymbolTable[instance.Type().(*types.PointerType).ElemType.Name()+"."+c.MethodName].(*ir.Func)
	if method == nil {
		panic(fmt.Errorf("method '%s.%s' not found", instance.Type().(*types.PointerType).ElemType.Name(), c.MethodName))
	}
	// Get the function arguments
	var args []value.Value
	for _, arg := range c.Args {
		args = append(args, ctx.compileExpr(arg))
	}
	// Call the function
	ctx.NewCall(method, append([]value.Value{instance}, args...)...)
}
