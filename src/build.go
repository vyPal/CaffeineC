package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/fatih/color"
	"github.com/urfave/cli/v2"
	"github.com/vyPal/CaffeineC/lib/analyzer"
	"github.com/vyPal/CaffeineC/lib/compiler"
	"github.com/vyPal/CaffeineC/lib/parser"
)

func init() {
	commands = append(commands, &cli.Command{
		Name:     "build",
		Usage:    "Build a CaffeineC file",
		Category: "compile",
		Flags: []cli.Flag{
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
			&cli.BoolFlag{
				Name:    "output-llvm",
				Aliases: []string{"l"},
				Usage:   "Output the LLVM IR to a file",
			},
			&cli.IntFlag{
				Name: "opt-level",
				Usage: "The optimization level to use. " +
					"0 is no optimization, 3 is the most optimization." +
					"Lower levels are faster to compile, higher levels are faster to run.",
				Aliases: []string{"O"},
				Value:   2,
			},
		},
		Action: build,
	},
		&cli.Command{
			Name:  "run",
			Usage: "Run a CaffeineC file",
			Flags: []cli.Flag{
				&cli.BoolFlag{
					Name:    "dump-ast",
					Aliases: []string{"d"},
					Usage:   "Dump the AST to a file",
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
				&cli.BoolFlag{
					Name:    "output-llvm",
					Aliases: []string{"l"},
					Usage:   "Output the LLVM IR to a file",
				},
			},
			Action: run,
		},
	)
}

func build(c *cli.Context) error {
	go checkUpdate(c)
	isWindows := runtime.GOOS == "windows"

	if c.Bool("ebnf") {
		fmt.Println(parser.Parser().String())
		return nil
	}

	outName := c.String("output")
	tmpDir, err := os.MkdirTemp("", "caffeinec")
	defer os.RemoveAll(tmpDir)
	if err != nil {
		return err
	}

	if outName == "" {
		if isWindows {
			outName = "output.exe"
		} else {
			outName = "output"
		}
	}

	f := c.Args().First()
	if f == "" {
		conf, err := GetCfConf(".")
		if err != nil {
			return err
		}
		f = conf.Main
	}

	inf, err := os.Stat(f)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	if os.IsNotExist(err) {
		return cli.Exit(color.RedString("File does not exist: %s", f), 1)
	} else if inf.IsDir() {
		conf, err := GetCfConf(f)
		if err != nil {
			return err
		}
		f = conf.Main
	}

	llData, req, err := parseAndCompile(f, tmpDir, c.Bool("dump-ast"))
	if err != nil {
		return err
	}

	/*
		if c.Bool("output-llvm") {
			err = os.WriteFile("output.ll", []byte(llData), 0644)
			if err != nil {
				return err
			}
		}

			oFile, err := llvmToObj(llData, tmpDir, c.Int("opt-level"))
			if err != nil {
				return err
			}
	*/

	imports, err := processIncludes(c.StringSlice("include"), req, tmpDir, c.Int("opt-level"))
	if err != nil {
		return err
	}

	args := append([]string{"clang", "-c", llData}, imports...)
	args = append(args, "-o", outName)
	cmd := exec.Command(args[0], args[1:]...)

	err = cmd.Run()
	if err != nil {
		log.Fatal(err)
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
	cmd.Stdin = os.Stdin
	err = cmd.Run()
	if err != nil {
		return cli.Exit(color.RedString("Error running binary: %s", err), 1)
	}

	return nil
}

func parseAndCompile(path, tmpdir string, dump bool) (string, []string, error) {
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
	go analyzer.Analyze(ast) // Removing this makes the compiler ~13ms faster

	comp := compiler.NewCompiler()
	wDir, err := filepath.Abs(filepath.Dir(path))
	if err != nil {
		return "", []string{}, err
	}
	req, err := comp.Compile(ast, wDir)
	if err != nil {
		return "", []string{}, cli.Exit(color.RedString("Error compiling: %s", err), 1)
	}

	f, err := os.CreateTemp(tmpdir, "caffeinec*.ll")
	if err != nil {
		return "", []string{}, err
	}
	defer f.Close()

	_, err = f.WriteString(comp.Module.String())
	if err != nil {
		return "", []string{}, err
	}

	return f.Name(), req, nil
}

func llvmToObj(llData, tmpdir string, opt int) (string, error) {
	randFileName := fmt.Sprintf("caffeinec%d", rand.Int())
	objPath := filepath.Join(tmpdir, randFileName)

	if runtime.GOOS == "windows" {
		args := []string{"clang -c -O" + fmt.Sprint(opt) + " - -o" + objPath}
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Stdin = strings.NewReader(llData)
		err := cmd.Run()
		if err != nil {
			return objPath, err
		}
	} else {
		cmd := exec.Command("sh", "-c", "llc -filetype=obj -O"+fmt.Sprint(opt)+" - -o "+objPath)
		cmd.Stdin = strings.NewReader(llData)
		err := cmd.Run()
		if err != nil {
			return objPath, err
		}
	}

	return objPath, nil
}

func processIncludes(includes []string, requirements []string, tmpDir string, opt int) ([]string, error) {
	var files []string
	includes = append(includes, requirements...)

	var wg sync.WaitGroup
	errs := make(chan error, 1)

	for _, include := range includes {
		info, err := os.Stat(include)
		if err != nil {
			return nil, err
		}

		if info.IsDir() {
			err = filepath.Walk(include, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}

				if !info.IsDir() {
					err = processFile(path, &files, tmpDir, opt, &wg, errs)
					if err != nil {
						return err
					}
				}

				return nil
			})
		} else {
			err = processFile(include, &files, tmpDir, opt, &wg, errs)
		}

		if err != nil {
			return nil, err
		}
	}

	go func() {
		wg.Wait()
		close(errs)
	}()

	if err, ok := <-errs; ok {
		return nil, err
	}

	return files, nil
}

func processFile(path string, files *[]string, tmpDir string, opt int, wg *sync.WaitGroup, errs chan<- error) error {
	ext := filepath.Ext(path)
	if ext == ".c" || ext == ".cpp" || ext == ".h" || ext == ".o" {
		*files = append(*files, path)
	} else if ext == ".cffc" {
		wg.Add(1)
		go func(path string) {
			defer wg.Done()
			llFile, req, err := parseAndCompile(path, tmpDir, false)
			if err != nil {
				errs <- err
				return
			}

			if len(req) > 0 {
				_, err := processIncludes([]string{}, req, tmpDir, opt)
				if err != nil {
					errs <- err
					return
				}
			}

			*files = append(*files, llFile)
		}(path)
	}

	return nil
}
