package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
			&cli.IntFlag{
				Name: "opt-level",
				Usage: "The optimization level to use. " +
					"0 is no optimization, 3 is the most optimization." +
					"Lower levels are faster to compile, higher levels are faster to run.",
				Aliases: []string{"O"},
				Value:   2,
			},
			&cli.BoolFlag{
				Name:    "debug",
				Usage:   "Save additional build files for debugging. ",
				Aliases: []string{"d"},
			},
		},
		Action: build,
	},
		&cli.Command{
			Name:  "run",
			Usage: "Run a CaffeineC file",
			Flags: []cli.Flag{
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
					Name:    "debug",
					Usage:   "Save additional build files for debugging. ",
					Aliases: []string{"d"},
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

	var conf CfConf

	f := c.Args().First()
	if f == "" {
		conf, err = GetCfConf(".")
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

	llData, req, err := parseAndCompile(f, tmpDir, c.Bool("debug"))
	if err != nil {
		return err
	}

	imports, err := processIncludes(c.StringSlice("include"), req, tmpDir, c.Int("opt-level"), c.Bool("debug"))
	if err != nil {
		return err
	}

	var stderr bytes.Buffer

	args := append([]string{"clang", llData}, imports...)
	args = append(args, "-o", outName)
	cmd := exec.Command(args[0], args[1:]...)

	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		log.Print("stderr:", stderr.String())
		log.Fatal(err)
	}

	// If c.Bool('debug') copy the tmpdir's contents to a new folder called debug in the current directory
	if c.Bool("debug") {
		err = os.Mkdir("debug", 0755)
		if err != nil {
			return err
		}

		cmd := exec.Command("cp", "-r", tmpDir, "debug")
		err = cmd.Run()
		if err != nil {
			return err
		}
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
		cwd, err := os.Getwd()
		if err != nil {
			return "", []string{}, cli.Exit(color.RedString("Error getting current working directory: %s", err), 1)
		}

		relativePath, err := filepath.Rel(cwd, path)
		if err != nil {
			relativePath = path
		}

		fullPath := filepath.Join(tmpdir, "ast-"+relativePath+".json")
		dirPath := filepath.Dir(fullPath)

		err = os.MkdirAll(dirPath, 0755)
		if err != nil {
			return "", []string{}, cli.Exit(color.RedString("Error creating directories: %s", err), 1)
		}

		astFile, err := os.Create(fullPath)
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

func processIncludes(includes []string, requirements []string, tmpDir string, opt int, dump bool) ([]string, error) {
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
					err = processFile(path, &files, tmpDir, opt, dump, &wg, errs)
					if err != nil {
						return err
					}
				}

				return nil
			})
		} else {
			err = processFile(include, &files, tmpDir, opt, dump, &wg, errs)
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

func processFile(path string, files *[]string, tmpDir string, opt int, dump bool, wg *sync.WaitGroup, errs chan<- error) error {
	ext := filepath.Ext(path)
	if ext == ".c" || ext == ".cpp" || ext == ".h" || ext == ".o" {
		*files = append(*files, path)
	} else if ext == ".cffc" {
		wg.Add(1)
		go func(path string) {
			defer wg.Done()
			llFile, req, err := parseAndCompile(path, tmpDir, dump)
			if err != nil {
				errs <- err
				return
			}

			if len(req) > 0 {
				_, err := processIncludes([]string{}, req, tmpDir, opt, dump)
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
