package analyzer

import (
	"strings"

	"github.com/vyPal/CaffeineC/lib/parser"
)

func ScanSymbols(statements []*parser.Statement) (map[string]string, []string, error) {
	symbols := make(map[string]string)
	var imports []string
	for _, statement := range statements {
		if statement.ClassDefinition != nil {
			symbols[statement.ClassDefinition.Name] = "class"
		}
		if statement.FunctionDefinition != nil {
			symbols[strings.Trim(statement.FunctionDefinition.Name.Name, "\"")] = "function"
		}
		if statement.External != nil {
			symbols[statement.External.Name] = "function"
		}
		if statement.VariableDefinition != nil {
			symbols[statement.VariableDefinition.Name] = "variable"
		}
		if statement.Import != nil {
			imports = append(imports, statement.Import.Package)
			if statement.Import.Alias != "" {
				symbols[statement.Import.Alias] = "package"
			} else {
				ast := parser.ParseFile(statement.Import.Package)
				symbols[ast.Package] = "package"
			}
		}
		if statement.FromImport != nil {
			imports = append(imports, statement.FromImport.Package)
			ast := parser.ParseFile(statement.FromImport.Package)
			storeSymbol := statement.FromImport.Symbol
			if statement.FromImport.Alias != "" {
				storeSymbol = statement.FromImport.Alias
			}
			for _, stat := range ast.Statements {
				if stat.Export != nil {
					if stat.Export.ClassDefinition != nil {
						if stat.Export.ClassDefinition.Name == statement.FromImport.Symbol {
							symbols[storeSymbol] = "class"
						}
					}
					if stat.Export.FunctionDefinition != nil {
						if stat.Export.FunctionDefinition.Name.Name == statement.FromImport.Symbol {
							symbols[storeSymbol] = "function"
						}
					}
					if stat.Export.External != nil {
						if stat.Export.External.Name == statement.FromImport.Symbol {
							symbols[storeSymbol] = "function"
						}
					}
					if stat.Export.VariableDefinition != nil {
						if stat.Export.VariableDefinition.Name == statement.FromImport.Symbol {
							symbols[storeSymbol] = "variable"
						}
					}
				}
			}
		}
		if statement.FromImportMultiple != nil {
			imports = append(imports, statement.FromImportMultiple.Package)
			ast := parser.ParseFile(statement.FromImportMultiple.Package)
			for _, symbol := range statement.FromImportMultiple.Symbols {
				storeSymbol := symbol.Name
				if symbol.Alias != "" {
					storeSymbol = symbol.Alias
				}
				for _, stat := range ast.Statements {
					if stat.Export != nil {
						if stat.Export.ClassDefinition != nil {
							if stat.Export.ClassDefinition.Name == symbol.Name {
								symbols[storeSymbol] = "class"
							}
						}
						if stat.Export.FunctionDefinition != nil {
							if stat.Export.FunctionDefinition.Name.Name == symbol.Name {
								symbols[storeSymbol] = "function"
							}
						}
						if stat.Export.External != nil {
							if stat.Export.External.Name == symbol.Name {
								symbols[storeSymbol] = "function"
							}
						}
						if stat.Export.VariableDefinition != nil {
							if stat.Export.VariableDefinition.Name == symbol.Name {
								symbols[storeSymbol] = "variable"
							}
						}
					}
				}
			}
		}
	}
	return symbols, imports, nil
}
