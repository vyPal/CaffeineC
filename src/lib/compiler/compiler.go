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
	parent        *Context
	vars          map[string]*Variable
	structNames   map[*types.StructType]string
	fc            *FlowControl
	RequestedType types.Type
	DestPtr       value.Value
	StoredInDest  bool
}

type Variable struct {
	Name  string
	Type  types.Type
	Value value.Value
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
		vars:        make(map[string]*Variable),
		structNames: make(map[*types.StructType]string),
		fc:          &FlowControl{},
	}
}

func (c *Context) NewContext(b *ir.Block) *Context {
	ctx := NewContext(b, c.Compiler)
	ctx.parent = c
	return ctx
}

func (c Context) lookupVariable(name string) *Variable {
	if c.Block != nil && c.Block.Parent != nil {
		for _, param := range c.Block.Parent.Params {
			if param.Name() == name {
				return &Variable{
					Name:  param.Name(),
					Type:  param.Type(),
					Value: param,
				}
			}
		}
	}
	if v, ok := c.vars[name]; ok {
		return v
	} else if c.parent != nil {
		v := c.parent.lookupVariable(name)
		return v
	} else {
		cli.Exit(color.RedString("Error: Unable to find a variable named: %s", name), 1)
	}
	return nil
}

func (c *Context) lookupFunction(name string) (*ir.Func, bool) {
	fn, ok := c.Compiler.SymbolTable[name]
	if ok {
		return fn.(*ir.Func), true
	} else {
		for _, f := range c.Module.Funcs {
			if f.Name() == name {
				return f, true
			}
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

func (c *Compiler) Init(program *parser.Program, workingDir string) {
	c.AST = program
	c.workingDir = workingDir
	c.Context = &Context{
		Compiler:    c,
		parent:      nil,
		vars:        make(map[string]*Variable),
		structNames: make(map[*types.StructType]string),
		fc:          &FlowControl{},
	}
}

func (c *Compiler) Compile() (err error) {
	for _, s := range c.AST.Statements {
		err := c.Context.compileStatement(s)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *Compiler) ListImportedFiles() (requiredImports []string, err error) {
	for _, s := range c.AST.Statements {
		if s.Import != nil {
			path := strings.Trim(s.Import.Package, "\"")
			_, importpath, err := ResolveImportPath(path, c.PackageCache)
			if err != nil {
				return []string{}, err
			}
			if !filepath.IsAbs(importpath) {
				importpath = filepath.Clean(filepath.Join(c.workingDir, importpath))
			}
			requiredImports = append(requiredImports, importpath)
		} else if s.FromImport != nil {
			path := strings.Trim(s.FromImport.Package, "\"")
			_, importpath, err := ResolveImportPath(path, c.PackageCache)
			if err != nil {
				return []string{}, err
			}
			if !filepath.IsAbs(importpath) {
				importpath = filepath.Clean(filepath.Join(c.workingDir, importpath))
			}
			requiredImports = append(requiredImports, importpath)
		} else if s.FromImportMultiple != nil {
			path := strings.Trim(s.FromImportMultiple.Package, "\"")
			_, importpath, err := ResolveImportPath(path, c.PackageCache)
			if err != nil {
				return []string{}, err
			}
			if !filepath.IsAbs(importpath) {
				importpath = filepath.Clean(filepath.Join(c.workingDir, importpath))
			}
			requiredImports = append(requiredImports, importpath)
		}
	}
	return requiredImports, nil
}

func (c *Compiler) FindImports() error {
	for i := len(c.AST.Statements) - 1; i >= 0; i-- {
		s := c.AST.Statements[i]
		if s.Import != nil {
			err := c.ImportAll(s.Import.Package, c.Context)
			if err != nil {
				return err
			}
			c.AST.Statements = append(c.AST.Statements[:i], c.AST.Statements[i+1:]...)
		} else if s.FromImport != nil {
			symbols := map[string]string{strings.Trim(s.FromImport.Symbol, "\""): strings.Trim(s.FromImport.Symbol, "\"")}
			err := c.ImportAs(s.FromImport.Package, symbols, c.Context)
			if err != nil {
				return err
			}
			c.AST.Statements = append(c.AST.Statements[:i], c.AST.Statements[i+1:]...)
		} else if s.FromImportMultiple != nil {
			symbols := map[string]string{}
			for _, symbol := range s.FromImportMultiple.Symbols {
				if symbol.Alias == "" {
					symbol.Alias = symbol.Name
				}
				symbols[strings.Trim(symbol.Name, "\"")] = strings.Trim(symbol.Alias, "\"")
			}
			err := c.ImportAs(s.FromImportMultiple.Package, symbols, c.Context)
			if err != nil {
				return err
			}
			c.AST.Statements = append(c.AST.Statements[:i], c.AST.Statements[i+1:]...)
		}
	}
	return nil
}

func (c *Compiler) ImportAll(path string, ctx *Context) error {
	path = strings.Trim(path, "\"")
	path, importpath, err := ResolveImportPath(path, c.PackageCache)
	if err != nil {
		return err
	}
	if !filepath.IsAbs(path) {
		path = filepath.Clean(filepath.Join(c.workingDir, path))
	}
	if !filepath.IsAbs(importpath) {
		importpath = filepath.Clean(filepath.Join(c.workingDir, importpath))
	}
	c.RequiredImports = append(c.RequiredImports, importpath)
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
					params = append(params, ir.NewParam(p.Name, ctx.CFTypeToLLType(p.Type)))
				}
				fn := c.Module.NewFunc(s.Export.FunctionDefinition.Name.Name, ctx.CFMultiTypeToLLType(s.Export.FunctionDefinition.ReturnType), params...)
				if s.Export.FunctionDefinition.Variadic != "" {
					fn.Sig.Variadic = true
				}
				ctx.SymbolTable[s.Export.FunctionDefinition.Name.Name] = fn

			} else if s.Export.ClassDefinition != nil {
				cStruct := types.NewStruct()
				cStruct.SetName(s.Export.ClassDefinition.Name)
				ctx.Module.NewTypeDef(s.Export.ClassDefinition.Name, cStruct)
				ctx.structNames[cStruct] = s.Export.ClassDefinition.Name
				for _, st := range s.Export.ClassDefinition.Body {
					if st.FieldDefinition != nil {
						cStruct.Fields = append(cStruct.Fields, ctx.CFTypeToLLType(st.FieldDefinition.Type))
						ctx.Compiler.StructFields[s.Export.ClassDefinition.Name] = append(ctx.Compiler.StructFields[s.Export.ClassDefinition.Name], st.FieldDefinition)
					} else if st.FunctionDefinition != nil {
						f := st.FunctionDefinition
						var params []*ir.Param
						params = append(params, ir.NewParam("this", types.NewPointer(cStruct)))
						for _, arg := range f.Parameters {
							params = append(params, ir.NewParam(arg.Name, ctx.CFTypeToLLType(arg.Type)))
						}

						ms := "." + f.Name.Name
						if f.Name.Op {
							ms = ".op." + strings.Trim(f.Name.Name, "\"")
						} else if f.Name.Get {
							ms = ".get." + strings.Trim(f.Name.Name, "\"")
						} else if f.Name.Set {
							ms = ".set." + strings.Trim(f.Name.Name, "\"")
						}

						fn := ctx.Module.NewFunc(s.Export.ClassDefinition.Name+ms, ctx.CFMultiTypeToLLType(f.ReturnType), params...)
						if st.FunctionDefinition.Variadic != "" {
							fn.Sig.Variadic = true
						}

						ctx.SymbolTable[s.Export.ClassDefinition.Name+ms] = fn
					}
				}
			} else if s.Export.External != nil {
				var params []*ir.Param
				for _, p := range s.Export.External.Parameters {
					params = append(params, ir.NewParam(p.Name, ctx.CFTypeToLLType(p.Type)))
				}
				fn := c.Module.NewFunc(s.Export.External.Name, ctx.CFMultiTypeToLLType(s.Export.External.ReturnType), params...)
				fn.Sig.Variadic = s.Export.External.Variadic
				ctx.SymbolTable[s.Export.External.Name] = fn
			} else {
				continue
			}
		}
	}
	return nil
}

func (c *Compiler) ImportAs(path string, symbols map[string]string, ctx *Context) error {
	path = strings.Trim(path, "\"")
	path, importpath, err := ResolveImportPath(path, c.PackageCache)
	if err != nil {
		return err
	}
	if !filepath.IsAbs(path) {
		path = filepath.Clean(filepath.Join(c.workingDir, path))
	}
	if !filepath.IsAbs(importpath) {
		importpath = filepath.Clean(filepath.Join(c.workingDir, importpath))
	}
	c.RequiredImports = append(c.RequiredImports, importpath)
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
				if newname, ok := symbols[s.Export.FunctionDefinition.Name.Name]; ok {
					var params []*ir.Param
					for _, p := range s.Export.FunctionDefinition.Parameters {
						params = append(params, ir.NewParam(p.Name, ctx.CFTypeToLLType(p.Type)))
					}
					fn := c.Module.NewFunc(s.Export.FunctionDefinition.Name.Name, ctx.CFMultiTypeToLLType(s.Export.FunctionDefinition.ReturnType), params...)
					if newname == "" {
						newname = s.Export.FunctionDefinition.Name.Name
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
							cStruct.Fields = append(cStruct.Fields, ctx.CFTypeToLLType(st.FieldDefinition.Type))
						} else if st.FunctionDefinition != nil {
							var params []*ir.Param
							for _, p := range st.FunctionDefinition.Parameters {
								params = append(params, ir.NewParam(p.Name, ctx.CFTypeToLLType(p.Type)))
							}
							f := st.FunctionDefinition

							ms := "." + f.Name.Name
							if f.Name.Op {
								ms = ".op." + strings.Trim(f.Name.Name, "\"")
							} else if f.Name.Get {
								ms = ".get." + strings.Trim(f.Name.Name, "\"")
							} else if f.Name.Set {
								ms = ".set." + strings.Trim(f.Name.Name, "\"")
							}

							fn := ctx.Module.NewFunc(s.Export.ClassDefinition.Name+ms, ctx.CFMultiTypeToLLType(f.ReturnType), params...)
							if st.FunctionDefinition.Variadic != "" {
								fn.Sig.Variadic = true
							}

							ctx.SymbolTable[s.Export.ClassDefinition.Name+ms] = fn
						}
					}
					ctx.structNames[cStruct] = newname
					ctx.Module.NewTypeDef(newname, cStruct)
				}
			} else if s.Export.External != nil {
				var params []*ir.Param
				for _, p := range s.Export.External.Parameters {
					params = append(params, ir.NewParam(p.Name, ctx.CFTypeToLLType(p.Type)))
				}
				fn := c.Module.NewFunc(s.Export.External.Name, ctx.CFMultiTypeToLLType(s.Export.External.ReturnType), params...)
				ctx.SymbolTable[s.Export.External.Name] = fn
			} else {
				continue
			}
		}
	}
	return nil
}
