package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/llir/llvm/ir"
	"github.com/llir/llvm/ir/constant"
	"github.com/vyPal/CaffeineC/lexer"
	"github.com/vyPal/CaffeineC/parser"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: ./main <command> [<args>]")
		os.Exit(1)
	}

	var src []byte
	var filename string
	var no_cleanup *bool
	switch os.Args[1] {
	case "build":
		buildCmd := flag.NewFlagSet("build", flag.ExitOnError)
		no_cleanup = buildCmd.Bool("n", false, "Don't remove temporary files")

		// Parse the flags for the build command
		buildCmd.Parse(os.Args[2:])

		if buildCmd.NArg() < 1 {
			fmt.Println("Usage: ./main build [-n] <filename>")
			os.Exit(1)
		}

		filename = buildCmd.Arg(0)

		code, err := os.ReadFile(filename)
		if err != nil {
			fmt.Println("Error reading file:", err)
			os.Exit(1)
		}
		src = code

		// Use src and *noCleanup here...

	default:
		fmt.Println("Unknown command:", os.Args[1])
		fmt.Println("Usage: ./main <command> [<args>]")
		os.Exit(1)
	}

	l := lexer.Lexer{}
	l.S.Init(strings.NewReader(string(src)))
	l.S.Filename = filename
	Tokens := l.Lex()

	mod := ir.NewModule()

	p := parser.Parser{Tokens: Tokens, Module: mod, SymbolTable: make(map[string]constant.Constant)}
	p.Parse()

	err := os.WriteFile("output.ll", []byte(p.Module.String()), 0644)
	if err != nil {
		log.Fatal(err)
	}

	// Compile the C code into an object file
	cmd := exec.Command("gcc", "-c", "./c_files/sleep.c", "-o", "sleep.o")
	err = cmd.Run()
	if err != nil {
		log.Fatal(err)
	}

	// Build the Go code
	cmd = exec.Command("llc", "-filetype=obj", "output.ll")
	err = cmd.Run()
	if err != nil {
		log.Fatal(err)
	}

	// Link everything together
	cmd = exec.Command("gcc", "output.o", "sleep.o", "-o", "output")
	err = cmd.Run()
	if err != nil {
		log.Fatal(err)
	}

	// Remove the temporary files
	fmt.Println(*no_cleanup)
	if !*no_cleanup {
		os.Remove("output.ll")
		os.Remove("output.o")
		os.Remove("sleep.o")
	}
}
