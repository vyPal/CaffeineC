package parser

import (
	"github.com/vyPal/CaffeineC/compiler"
	"github.com/vyPal/CaffeineC/lexer"
)

type Parser struct {
	Tokens []lexer.Token
	Pos    int
	AST    []compiler.Stmt
}

func (p *Parser) Parse() {
	for p.Pos < len(p.Tokens) {
		p.parseStatement()
	}
}
