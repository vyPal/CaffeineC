package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/fatih/color"
	"github.com/urfave/cli/v2"
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
						Name: "ebnf",
						Usage: "Print the EBNF grammar for CaffeineC. " +
							"Useful for debugging the parser.",
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

	if c.Bool("ebnf") {
		fmt.Println(parser.Parser().String())
		return nil
	}

	llcName := "llc"
	optName := "opt"
	outName := c.String("output")
	tmpDir := "tmp_compile"

	if isWindows {
		if outName == "" {
			outName = "output.exe"
		}
		llcName = tmpDir + "/llc.exe"
		err := os.WriteFile(llcName, llcExe, 0755)
		if err != nil {
			panic(err)
		}
		optName = tmpDir + "/opt.exe"
		err = os.WriteFile(optName, optExe, 0755)
		if err != nil {
			panic(err)
		}
	}
	if outName == "" {
		outName = "output"
	}
	err := os.Mkdir(tmpDir, 0755)
	if err != nil && !os.IsExist(err) {
		return cli.Exit(color.RedString("Error creating temporary directory: %s", err), 1)
	}

	llFile, req, err := parseAndCompile(c.Args().First(), tmpDir, c.Bool("dump-ast"), true)
	if err != nil {
		return err
	}

	oFile, err := llvmToObj(llFile, tmpDir, llcName, optName, c.Bool("no-optimization"))
	if err != nil {
		return err
	}

	imports, err := processIncludes(c.StringSlice("include"), req, tmpDir, llcName, optName)
	if err != nil {
		return err
	}

	args := append([]string{"gcc", oFile}, imports...)
	args = append(args, "-o", outName)
	cmd := exec.Command(args[0], args[1:]...)

	err = cmd.Run()
	if err != nil {
		log.Fatal(err)
	}

	// Remove the temporary files

	if !c.Bool("no-cleanup") {
		os.RemoveAll(tmpDir)
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

func parseAndCompile(path, tmpdir string, dump, isMain bool) (string, []string, error) {
	ast := parser.ParseFile(path)
	if dump {
		astFile, err := os.Create("ast_dump.json")
		if err != nil {
			return "", []string{}, cli.Exit(color.RedString("Error creating AST dump file: %s", err), 1)
		}
		defer astFile.Close()

		encoder := json.NewEncoder(astFile)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(ast); err != nil {
			return "", []string{}, cli.Exit(color.RedString("Error encoding AST: %s", err), 1)
		}
	}
	//go analyzer.Analyze(ast) // Removing this makes the compiler ~13ms faster

	comp := compiler.NewCompiler()
	wDir, err := filepath.Abs(filepath.Dir(path))
	if err != nil {
		return "", []string{}, err
	}
	req, err := comp.Compile(ast, wDir, isMain)
	if err != nil {
		return "", []string{}, cli.Exit(color.RedString("Error compiling: %s", err), 1)
	}

	newPath := filepath.Join(tmpdir, filepath.Base(path)+".ll")

	return newPath, req, os.WriteFile(newPath, []byte(comp.Module.String()), 0644)
}

func llvmToObj(path, tmpdir, llc, opt string, noopt bool) (string, error) {
	objPath := filepath.Join(tmpdir, filepath.Base(path)+".o")
	if noopt {
		cmd := exec.Command(llc, path, "-filetype=obj", "-o", objPath)
		err := cmd.Run()
		if err != nil {
			return objPath, err
		}
	} else {
		bitCodePath := filepath.Join(tmpdir, filepath.Base(path)+".bc")
		cmd := exec.Command(opt, path, "-o", bitCodePath)
		err := cmd.Run()
		if err != nil {
			return objPath, err
		}

		cmd = exec.Command(llc, bitCodePath, "-filetype=obj", "-o", objPath)
		err = cmd.Run()
		if err != nil {
			return objPath, err
		}
	}
	return objPath, nil
}

func processIncludes(includes []string, requirements []string, tmpDir, llcName, optName string) ([]string, error) {
	var files []string
	includes = append(includes, requirements...)

	for _, include := range includes {
		err := filepath.Walk(include, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if info.IsDir() {
				return nil
			}

			ext := filepath.Ext(path)
			if ext == ".c" || ext == ".cpp" || ext == ".h" || ext == ".o" {
				files = append(files, path)
			} else if ext == ".cffc" {
				llFile, req, err := parseAndCompile(path, tmpDir, false, false)
				if err != nil {
					return err
				}

				if len(req) > 0 {
					processIncludes([]string{}, req, tmpDir, llcName, optName)
				}

				oFile, err := llvmToObj(llFile, tmpDir, llcName, optName, true)
				if err != nil {
					return err
				}

				files = append(files, oFile)
			}

			return nil
		})

		if err != nil {
			return nil, err
		}
	}

	return files, nil
}
