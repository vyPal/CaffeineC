package analyzer

import "github.com/vyPal/CaffeineC/lib/parser"

type Analyzer struct {
	symbols map[string]string
	imports []string
	ast     *parser.Program
}

func NewAnalyzer() *Analyzer {
	return &Analyzer{
		symbols: make(map[string]string),
		imports: make([]string, 0),
	}
}

func (a *Analyzer) Analyze() (err error) {
	return err
}

func (a *Analyzer) Scan(ast *parser.Program) (err error) {
	a.symbols, a.imports, err = ScanSymbols(ast.Statements)
	a.ast = ast
	return err
}
