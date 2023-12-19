package parser

import (
	"os"

	"github.com/alecthomas/participle/v2"
)

func ParseFile(filename string) *Program {
	parser := participle.MustBuild[Program]()

	file, err := os.ReadFile(filename)
	if err != nil {
		panic(err)
	}

	ast, err := parser.ParseString(filename, string(file))
	if err != nil {
		panic(err)
	}
	return ast
}

func Parser() *participle.Parser[Program] {
	return participle.MustBuild[Program]()
}

func ParseString(code string) *Program {
	parser := participle.MustBuild[Program]()

	ast, err := parser.ParseString("", code)
	if err != nil {
		panic(err)
	}
	return ast
}
