package cflex

import (
	"bytes"
	"io"
	"strings"
	"text/scanner"

	"github.com/alecthomas/participle/v2"
	"github.com/alecthomas/participle/v2/lexer"
)

// TextScannerLexer is a lexer that uses the text/scanner module.
var (
	TextScannerLexer lexer.Definition = &textScannerLexerDefinition{}

	// DefaultDefinition defines properties for the default lexer.
	DefaultDefinition = TextScannerLexer
)

// NewTextScannerLexer constructs a Definition that uses an underlying scanner.Scanner
//
// "configure" will be called after the scanner.Scanner.Init(r) is called. If "configure"
// is nil a default scanner.Scanner will be used.
func NewTextScannerLexer(configure func(*scanner.Scanner)) lexer.Definition {
	return &textScannerLexerDefinition{configure: configure}
}

type textScannerLexerDefinition struct {
	configure func(*scanner.Scanner)
}

func (d *textScannerLexerDefinition) Lex(filename string, r io.Reader) (lexer.Lexer, error) {
	l := Lex(filename, r)
	if d.configure != nil {
		d.configure(l.(*textScannerLexer).scanner)
	}
	return l, nil
}

func (d *textScannerLexerDefinition) Symbols() map[string]lexer.TokenType {
	return map[string]lexer.TokenType{
		"EOF":       lexer.EOF,
		"Char":      scanner.Char,
		"Ident":     scanner.Ident,
		"Int":       scanner.Int,
		"Float":     scanner.Float,
		"String":    scanner.String,
		"RawString": scanner.RawString,
		"Comment":   scanner.Comment,
	}
}

// textScannerLexer is a Lexer based on text/scanner.Scanner
type textScannerLexer struct {
	scanner  *scanner.Scanner
	filename string
	err      error
}

// Lex an io.Reader with text/scanner.Scanner.
//
// This provides very fast lexing of source code compatible with Go tokens.
//
// Note that this differs from text/scanner.Scanner in that string tokens will be unquoted.
func Lex(filename string, r io.Reader) lexer.Lexer {
	s := &scanner.Scanner{}
	s.Init(r)
	s.Mode &^= scanner.SkipComments
	lexerr := lexWithScanner(filename, s)
	lexerr.scanner.Error = func(s *scanner.Scanner, msg string) {
		lexerr.err = participle.Errorf(lexer.Position(lexerr.scanner.Pos()), msg)
	}
	return lexerr
}

// LexWithScanner creates a Lexer from a user-provided scanner.Scanner.
//
// Useful if you need to customise the Scanner.
func LexWithScanner(filename string, scan *scanner.Scanner) lexer.Lexer {
	return lexWithScanner(filename, scan)
}

func lexWithScanner(filename string, scan *scanner.Scanner) *textScannerLexer {
	scan.Filename = filename
	lexer := &textScannerLexer{
		filename: filename,
		scanner:  scan,
	}
	return lexer
}

// LexBytes returns a new default lexer over bytes.
func LexBytes(filename string, b []byte) lexer.Lexer {
	return Lex(filename, bytes.NewReader(b))
}

// LexString returns a new default lexer over a string.
func LexString(filename, s string) lexer.Lexer {
	return Lex(filename, strings.NewReader(s))
}

func (t *textScannerLexer) Next() (lexer.Token, error) {
	typ := t.scanner.Scan()
	text := t.scanner.TokenText()
	pos := lexer.Position(t.scanner.Position)
	pos.Filename = t.filename
	if t.err != nil {
		return lexer.Token{}, t.err
	}
	return lexer.Token{
		Type:  lexer.TokenType(typ),
		Value: text,
		Pos:   pos,
	}, nil
}
