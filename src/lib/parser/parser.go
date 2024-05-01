package parser

import (
	"os"

	"github.com/alecthomas/participle/v2"
	cflex "github.com/vyPal/CaffeineC/lib/lexer"
)

var parser *participle.Parser[Program]
var parsed map[string]*Program

func ParseFile(filename string) *Program {
	if parsed == nil {
		parsed = make(map[string]*Program)
	}

	if parsed[filename] != nil {
		return parsed[filename]
	}

	if parser == nil {
		parser = participle.MustBuild[Program](participle.Lexer(cflex.DefaultDefinition))
	}

	file, err := os.ReadFile(filename)
	if err != nil {
		panic(err)
	}

	ast, err := parser.ParseString(filename, string(file))
	if err != nil {
		panic(err)
	}
	parsed[filename] = ast
	return ast
}

func Parser() *participle.Parser[Program] {
	if parser == nil {
		parser = participle.MustBuild[Program](participle.Lexer(cflex.DefaultDefinition))
	}
	return parser
}

func ParseString(code string) *Program {
	if parser == nil {
		parser = participle.MustBuild[Program](participle.Lexer(cflex.DefaultDefinition))
	}

	ast, err := parser.ParseString("", code)
	if err != nil {
		panic(err)
	}
	return ast
}
