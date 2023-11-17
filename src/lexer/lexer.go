package lexer

import (
	"fmt"
	"text/scanner"
)

type Token struct {
	Type     string
	Value    string
	Location scanner.Position
}

type Lexer struct {
	S scanner.Scanner
}

func (l *Lexer) Lex() []Token {
	var Tokens []Token
	for tok := l.S.Scan(); tok != scanner.EOF; tok = l.S.Scan() {
		switch tok {
		case scanner.Ident:
			Tokens = append(Tokens, Token{"IDENT", l.S.TokenText(), l.S.Pos()})
		case scanner.Int, scanner.Float:
			Tokens = append(Tokens, Token{"NUMBER", l.S.TokenText(), l.S.Pos()})
		case scanner.String:
			Tokens = append(Tokens, Token{"STRING", l.S.TokenText(), l.S.Pos()})
		default:
			Tokens = append(Tokens, Token{"PUNCT", l.S.TokenText(), l.S.Pos()})
		}
	}
	fmt.Println(Tokens)
	return Tokens
}
