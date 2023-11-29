package parser

import (
	"os"

	"github.com/alecthomas/participle/v2"
)

func ParseFile(filename string) *Program {
	parser := participle.MustBuild[Program]()

	file, err := os.ReadFile("example.cffc")
	if err != nil {
		panic(err)
	}

	ast, err := parser.ParseString("example.cffc", string(file))
	if err != nil {
		panic(err)
	}
	return ast
}

func ParseString(code string) *Program {
	parser := participle.MustBuild[Program]()

	ast, err := parser.ParseString("", code)
	if err != nil {
		panic(err)
	}
	return ast
}
