package main

import (
	_ "embed"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/llir/llvm/ir"
	"github.com/llir/llvm/ir/value"
	"github.com/vyPal/CaffeineC/compiler"
	"github.com/vyPal/CaffeineC/lexer"
	"github.com/vyPal/CaffeineC/parser"
)

//go:embed c_files/sleep.c
var cSource string

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: ./CaffeineC <command> [<args>]")
		os.Exit(1)
	}

	var src []byte
	var filename string
	var no_cleanup *bool
	var numbers_are_variables *bool
	var output_file *string
	switch os.Args[1] {
	case "build":
		buildCmd := flag.NewFlagSet("build", flag.ExitOnError)
		no_cleanup = buildCmd.Bool("nc", false, "Don't remove temporary files")
		numbers_are_variables = buildCmd.Bool("nv", false, "Allow variable names to be numbers")
		output_file = buildCmd.String("o", "output", "The name for the built binary")

		// Parse the flags for the build command
		buildCmd.Parse(os.Args[2:])

		if buildCmd.NArg() < 1 {
			fmt.Println("Usage: ./CaffeineC build [-nc | -nv] <filename>")
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
		fmt.Println("Usage: ./CaffeineC <command> [<args>]")
		os.Exit(1)
	}

	l := lexer.Lexer{}
	l.S.Init(strings.NewReader(string(src)))
	l.S.Filename = filename
	Tokens := l.Lex()

	mod := ir.NewModule()

	p := parser.Parser{Tokens: Tokens}
	p.Parse()

	c := compiler.Compiler{Module: mod, SymbolTable: make(map[string]value.Value), AST: p.AST, VarsCanBeNumbers: *numbers_are_variables, StructFields: make(map[string][]compiler.Field)}
	c.Compile()

	tmpDir := "tmp_compile"
	err := os.Mkdir(tmpDir, 0755)
	if err != nil && !os.IsExist(err) {
		panic(err)
	}

	cFilePath := tmpDir + "/sleep.c"
	err = os.WriteFile(cFilePath, []byte(cSource), 0644)
	if err != nil {
		panic(err)
	}

	err = os.WriteFile("tmp_compile/output.ll", []byte(c.Module.String()), 0644)
	if err != nil {
		log.Fatal(err)
	}

	// Compile the C code into an object file
	cmd := exec.Command("gcc", "-c", cFilePath, "-o", tmpDir+"/sleep.o")
	err = cmd.Run()
	if err != nil {
		log.Fatal(err)
	}

	if runtime.GOOS == "windows" {
		// Save the embedded llc executable to a temporary file
		llcExePath := tmpDir + "/llc.exe"
		err := os.WriteFile(llcExePath, llcExe, 0755)
		if err != nil {
			panic(err)
		}
		optExePath := tmpDir + "/opt.exe"
		err = os.WriteFile(optExePath, optExe, 0755)
		if err != nil {
			panic(err)
		}

		// Use the embedded llc executable
		cmd := exec.Command(optExePath, tmpDir+"/output.ll", "-o", tmpDir+"/output.bc")
		err = cmd.Run()
		if err != nil {
			panic(err)
		}

		cmd = exec.Command(llcExePath, tmpDir+"/output.bc", "-filetype=obj", "-o", tmpDir+"/output.o")
		err = cmd.Run()
		if err != nil {
			panic(err)
		}
	} else {
		cmd := exec.Command("opt", tmpDir+"/output.ll", "-o", tmpDir+"/output.bc")
		err = cmd.Run()
		if err != nil {
			panic(err)
		}

		cmd = exec.Command("llc", tmpDir+"/output.bc", "-filetype=obj", "-o", tmpDir+"/output.o")
		err = cmd.Run()
		if err != nil {
			panic(err)
		}
	}

	// Link everything together
	cmd = exec.Command("gcc", tmpDir+"/output.o", tmpDir+"/sleep.o", "-o", tmpDir+"/output")
	err = cmd.Run()
	if err != nil {
		log.Fatal(err)
	}

	err = os.Rename(tmpDir+"/output", *output_file)
	if err != nil {
		log.Fatal(err)
	}

	// Remove the temporary files
	if !*no_cleanup {
		if runtime.GOOS == "windows" {
			os.Remove(tmpDir + "/llc.exe")
			os.Remove(tmpDir + "/opt.exe")
		}
		os.Remove(tmpDir + "/output.ll")
		os.Remove(tmpDir + "/output.bc")
		os.Remove(tmpDir + "/output.o")
		os.Remove(tmpDir + "/sleep.o")
		os.Remove(tmpDir + "/sleep.c")
		os.Remove(tmpDir)
	}

	fmt.Println("Done!")
}
