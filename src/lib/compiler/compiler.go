package compiler

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/llir/llvm/ir"
	"github.com/llir/llvm/ir/types"
	"github.com/llir/llvm/ir/value"
	"github.com/urfave/cli/v2"
	"github.com/vyPal/CaffeineC/lib/cache"
	"github.com/vyPal/CaffeineC/lib/parser"
)

type Context struct {
	*ir.Block
	*Compiler
	parent      *Context
	vars        map[string]value.Value
	usedVars    map[string]bool
	structNames map[*types.StructType]string
	fc          *FlowControl
}

type FlowControl struct {
	Leave    *ir.Block
	Continue *ir.Block
}

func NewContext(b *ir.Block, comp *Compiler) *Context {
	return &Context{
		Block:       b,
		Compiler:    comp,
		parent:      nil,
		vars:        make(map[string]value.Value),
		usedVars:    make(map[string]bool),
		structNames: make(map[*types.StructType]string),
		fc:          &FlowControl{},
	}
}

func (c *Context) NewContext(b *ir.Block) *Context {
	ctx := NewContext(b, c.Compiler)
	ctx.parent = c
	return ctx
}

func (c Context) lookupVariable(name string) value.Value {
	if c.Block != nil && c.Block.Parent != nil {
		for _, param := range c.Block.Parent.Params {
			if param.Name() == name {
				return param
			}
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
	fn, ok := c.Compiler.SymbolTable[name]
	if ok {
		return fn.(*ir.Func), true
	}
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
	Module          *ir.Module
	SymbolTable     map[string]value.Value
	StructFields    map[string][]*parser.FieldDefinition
	Context         *Context
	AST             *parser.Program
	workingDir      string
	RequiredImports []string
	PackageCache    cache.PackageCache
}

func NewCompiler() *Compiler {
	return &Compiler{
		Module:          ir.NewModule(),
		SymbolTable:     make(map[string]value.Value),
		StructFields:    make(map[string][]*parser.FieldDefinition),
		RequiredImports: make([]string, 0),
	}
}

func (c *Compiler) Compile(program *parser.Program, workingDir string) (needsImports []string, err error) {
	c.AST = program
	c.workingDir = workingDir
	c.Context = &Context{
		Compiler:    c,
		parent:      nil,
		vars:        make(map[string]value.Value),
		usedVars:    make(map[string]bool),
		structNames: make(map[*types.StructType]string),
		fc:          &FlowControl{},
	}
	for _, s := range program.Statements {
		err := c.Context.compileStatement(s)
		if err != nil {
			return []string{}, err
		}
	}
	return c.RequiredImports, nil
}

func (c *Compiler) ImportAll(path string, ctx *Context) error {
	path = strings.Trim(path, "\"")
	path, err := ResolveImportPath(path, c.PackageCache)
	if err != nil {
		return err
	}
	if !filepath.IsAbs(path) {
		path = filepath.Clean(filepath.Join(c.workingDir, path))
	}
	c.RequiredImports = append(c.RequiredImports, path)
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return cli.Exit(color.RedString("Unable to import directory"), 1)
	}
	ast := parser.ParseFile(path)
	for _, s := range ast.Statements {
		if s.Export != nil {
			if s.Export.FunctionDefinition != nil {
				var params []*ir.Param
				for _, p := range s.Export.FunctionDefinition.Parameters {
					params = append(params, ir.NewParam(p.Name, ctx.StringToType(p.Type)))
				}
				fn := c.Module.NewFunc(s.Export.FunctionDefinition.Name, ctx.StringToType(s.Export.FunctionDefinition.ReturnType), params...)
				ctx.SymbolTable[s.Export.FunctionDefinition.Name] = fn
			} else if s.Export.ClassDefinition != nil {
				cStruct := types.NewStruct()
				cStruct.SetName(s.Export.ClassDefinition.Name)
				ctx.Module.NewTypeDef(s.Export.ClassDefinition.Name, cStruct)
				ctx.structNames[cStruct] = s.Export.ClassDefinition.Name
				for _, st := range s.Export.ClassDefinition.Body {
					if st.FieldDefinition != nil {
						cStruct.Fields = append(cStruct.Fields, ctx.StringToType(st.FieldDefinition.Type))
						ctx.Compiler.StructFields[s.Export.ClassDefinition.Name] = append(ctx.Compiler.StructFields[s.Export.ClassDefinition.Name], st.FieldDefinition)
					} else if st.FunctionDefinition != nil {
						f := st.FunctionDefinition
						var params []*ir.Param
						params = append(params, ir.NewParam("this", types.NewPointer(cStruct)))
						for _, arg := range f.Parameters {
							params = append(params, ir.NewParam(arg.Name, ctx.StringToType(arg.Type)))
						}

						fn := ctx.Module.NewFunc(s.Export.ClassDefinition.Name+"."+f.Name, ctx.StringToType(f.ReturnType), params...)
						fn.Sig.Variadic = false
						fn.Sig.RetType = ctx.StringToType(f.ReturnType)

						ctx.SymbolTable[s.Export.ClassDefinition.Name+"."+f.Name] = fn
					}
				}
			} else {
				continue
			}
		}
	}
	return nil
}

func (c *Compiler) ImportAs(path string, symbols map[string]string, ctx *Context) error {
	path = strings.Trim(path, "\"")
	path, err := ResolveImportPath(path, c.PackageCache)
	if err != nil {
		return err
	}
	if !filepath.IsAbs(path) {
		path = filepath.Clean(filepath.Join(c.workingDir, path))
	}
	c.RequiredImports = append(c.RequiredImports, path)
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return cli.Exit(color.RedString("Unable to import directory"), 1)
	}
	ast := parser.ParseFile(path)
	for _, s := range ast.Statements {
		if s.Export != nil {
			if s.Export.FunctionDefinition != nil {
				if newname, ok := symbols[s.Export.FunctionDefinition.Name]; ok {
					var params []*ir.Param
					for _, p := range s.Export.FunctionDefinition.Parameters {
						params = append(params, ir.NewParam(p.Name, ctx.StringToType(p.Type)))
					}
					fn := c.Module.NewFunc(s.Export.FunctionDefinition.Name, ctx.StringToType(s.Export.FunctionDefinition.ReturnType), params...)
					if newname == "" {
						newname = s.Export.FunctionDefinition.Name
					}
					ctx.SymbolTable[newname] = fn
				}
			} else if s.Export.ClassDefinition != nil {
				if newname, ok := symbols[s.Export.ClassDefinition.Name]; ok {
					if newname == "" {
						newname = s.Export.ClassDefinition.Name
					}
					cStruct := types.NewStruct()
					for _, st := range s.Export.ClassDefinition.Body {
						if st.FieldDefinition != nil {
							cStruct.Fields = append(cStruct.Fields, ctx.StringToType(st.FieldDefinition.Type))
						} else if st.FunctionDefinition != nil {
							var params []*ir.Param
							for _, p := range st.FunctionDefinition.Parameters {
								params = append(params, ir.NewParam(p.Name, ctx.StringToType(p.Type)))
							}
							fn := c.Module.NewFunc(st.FunctionDefinition.Name, ctx.StringToType(st.FunctionDefinition.ReturnType), params...)
							ctx.SymbolTable[newname+"."+st.FunctionDefinition.Name] = fn
						}
					}
					ctx.structNames[cStruct] = newname
					ctx.Module.NewTypeDef(newname, cStruct)
				}
			} else {
				continue
			}
		}
	}
	return nil
}
