package parser

import (
	"github.com/llir/llvm/ir"
	"github.com/llir/llvm/ir/constant"
	"github.com/llir/llvm/ir/types"
	"github.com/vyPal/CaffeineC/lexer"
)

type Parser struct {
	Module            *ir.Module
	SymbolTable       map[string]constant.Constant
	Tokens            []lexer.Token
	Pos               int
	CurrentBlock      *ir.Block
	InternalFunctions map[string]*ir.Func
}

func (p *Parser) Parse() {
	f := p.Module.NewFunc("main", types.Void)
	p.CurrentBlock = f.NewBlock("main")
	p.registerInternalFunctions()

	for p.Pos < len(p.Tokens) {
		p.parseStatement()
	}
	p.CurrentBlock.NewRet(nil)
}
