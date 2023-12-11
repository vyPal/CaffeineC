package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"

	"github.com/fatih/color"
	"github.com/urfave/cli/v2"
	"github.com/vyPal/CaffeineC/lib/analyzer"
	"github.com/vyPal/CaffeineC/lib/compiler"
	"github.com/vyPal/CaffeineC/lib/parser"
)

//go:embed c_files/sleep.c
var cSource string

func main() {
	app := &cli.App{
		Name:                   "CaffeineC",
		Usage:                  "A C-like language that compiles to LLVM IR",
		EnableBashCompletion:   true,
		UseShortOptionHandling: true,
		Commands: []*cli.Command{
			{
				Name:  "build",
				Usage: "Build a CaffeineC file",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "no-cleanup",
						Aliases: []string{"c"},
						Usage:   "Don't remove temporary files",
					},
					&cli.BoolFlag{
						Name:    "dump-ast",
						Aliases: []string{"d"},
						Usage:   "Dump the AST to a file",
					},
					&cli.BoolFlag{
						Name:    "only-parse",
						Aliases: []string{"p"},
						Usage:   "Only parse the file and dump the AST to stdout",
					},
					&cli.BoolFlag{
						Name: "ebnf",
						Usage: "Print the EBNF grammar for CaffeineC. " +
							"Useful for debugging the parser.",
					},
					&cli.StringFlag{
						Name:    "input-str",
						Aliases: []string{"s"},
						Usage:   "Compile a string instead of a file",
					},
					&cli.BoolFlag{
						Name:    "no-optimization",
						Aliases: []string{"n"},
						Usage:   "Don't run the 'opt' command",
					},
					&cli.StringFlag{
						Name:    "output",
						Aliases: []string{"o"},
						Usage:   "The name for the built binary",
					},
					&cli.StringSliceFlag{
						Name:    "include",
						Aliases: []string{"i"},
						Usage:   "Add a directory or file to the include path",
					},
				},
				Action: build,
			},
			{
				Name:  "run",
				Usage: "Run a CaffeineC file",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:    "no-cleanup",
						Aliases: []string{"c"},
						Usage:   "Don't remove temporary files",
					},
					&cli.BoolFlag{
						Name:    "dump-ast",
						Aliases: []string{"d"},
						Usage:   "Dump the AST to a file",
					},
					&cli.BoolFlag{
						Name:    "only-parse",
						Aliases: []string{"p"},
						Usage:   "Only parse the file and dump the AST to stdout",
					},
					&cli.StringFlag{
						Name:    "input-str",
						Aliases: []string{"s"},
						Usage:   "Compile a string instead of a file",
					},
					&cli.BoolFlag{
						Name:    "no-optimization",
						Aliases: []string{"n"},
						Usage:   "Don't run the 'opt' command",
					},
					&cli.StringFlag{
						Name:    "output",
						Aliases: []string{"o"},
						Usage:   "The name for the built binary",
					},
					&cli.StringSliceFlag{
						Name:    "include",
						Aliases: []string{"i"},
						Usage:   "Add a directory or file to the include path",
					},
				},
				Action: run,
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		panic(err)
	}
}

func build(c *cli.Context) error {
	isWindows := runtime.GOOS == "windows"

	var ast *parser.Program

	if c.Bool("ebnf") {
		fmt.Println(parser.Parser().String())
		return nil
	}

	if c.String("input-str") != "" {
		ast = parser.ParseString(c.String("input-str"))
	} else {
		filename := c.Args().First()
		if filename == "" {
			return cli.Exit(color.RedString("Error: No file specified"), 1)
		}

		ast = parser.ParseFile(filename)
	}

	if c.Bool("only-parse") {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(ast); err != nil {
			return cli.Exit(color.RedString("Error encoding AST: %s", err), 1)
		}
		return nil
	}

	if c.Bool("dump-ast") {
		astFile, err := os.Create("ast_dump.json")
		if err != nil {
			return cli.Exit(color.RedString("Error creating AST dump file: %s", err), 1)
		}
		defer astFile.Close()

		encoder := json.NewEncoder(astFile)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(ast); err != nil {
			return cli.Exit(color.RedString("Error encoding AST: %s", err), 1)
		}
	}

	analyzer.Analyze(ast)

	comp := compiler.NewCompiler()
	err := comp.Compile(ast)
	if err != nil {
		return cli.Exit(color.RedString("Error compiling: %s", err), 1)
	}

	tmpDir := "tmp_compile"
	err = os.Mkdir(tmpDir, 0755)

	if err != nil && !os.IsExist(err) {
		return cli.Exit(color.RedString("Error creating temporary directory: %s", err), 1)
	}

	err = os.WriteFile("tmp_compile/output.ll", []byte(comp.Module.String()), 0644)

	if err != nil {
		log.Fatal(err)
	}

	if isWindows {
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
		if !c.Bool("no-optimization") {
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
			cmd := exec.Command(llcExePath, tmpDir+"/output.ll", "-filetype=obj", "-o", tmpDir+"/output.o")
			err = cmd.Run()
			if err != nil {
				panic(err)
			}
		}

		includes := c.StringSlice("include")

		args := append([]string{"gcc", tmpDir + "/output.o", "-o", tmpDir + "/output.exe"}, includes...)
		cmd := exec.Command(args[0], args[1:]...)

		err = cmd.Run()
		if err != nil {
			log.Fatal(err)
		}

		out := c.String("output")
		if out == "" {
			out = "output.exe"
		}
		err = os.Rename(tmpDir+"/output.exe", out)
		if err != nil {
			log.Fatal(err)
		}
	} else {

		if !c.Bool("no-optimization") {
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
		} else {
			cmd := exec.Command("llc", tmpDir+"/output.ll", "-filetype=obj", "-o", tmpDir+"/output.o")
			err = cmd.Run()
			if err != nil {
				panic(err)
			}
		}

		includes := c.StringSlice("include")

		args := append([]string{"gcc", tmpDir + "/output.o", "-o", tmpDir + "/output"}, includes...)
		cmd := exec.Command(args[0], args[1:]...)

		err = cmd.Run()
		if err != nil {
			log.Fatal(err)
		}

		out := c.String("output")
		if out == "" {
			out = "output"
		}
		err = os.Rename(tmpDir+"/output", out)
		if err != nil {
			log.Fatal(err)
		}
	}

	// Remove the temporary files

	if !c.Bool("no-cleanup") {
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

	return nil
}

func run(c *cli.Context) error {
	err := build(c)
	if err != nil {
		return err
	}

	out := c.String("output")
	if out == "" {
		out = "output"
	}
	cmd := exec.Command("./" + out)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return cli.Exit(color.RedString("Error running binary: %s", err), 1)
	}

	return nil
}
