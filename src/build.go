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
	"slices"
	"strings"
	"sync"

	"github.com/fatih/color"
	"github.com/llir/llvm/ir/types"
	"github.com/urfave/cli/v2"
	"github.com/vyPal/CaffeineC/lib/cache"
	"github.com/vyPal/CaffeineC/lib/compiler"
	"github.com/vyPal/CaffeineC/lib/parser"
	"github.com/vyPal/CaffeineC/lib/project"
)

func init() {
	commands = append(commands, &cli.Command{
		Name:     "build",
		Usage:    "Build a CaffeineC file",
		Category: "compile",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "config",
				Usage:   "The path to the config file. ",
				Aliases: []string{"c"},
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
				Name:    "debug",
				Usage:   "Save additional build files for debugging. ",
				Aliases: []string{"d"},
			},
			&cli.BoolFlag{
				Name:  "obj",
				Usage: "Compile to an object file instead of an executable. ",
			},
			&cli.BoolFlag{
				Name:  "header",
				Usage: "Generate .h files for each .cffc file.",
			},
			&cli.StringSliceFlag{
				Name:    "clang-args",
				Aliases: []string{"a"},
				Usage: "Pass additional arguments to clang. " +
					"Useful for passing flags like -O2 or -g.",
			},
		},
		Action: build,
	},
		&cli.Command{
			Name:     "run",
			Category: "compile",
			Usage:    "Run a CaffeineC file",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    "config",
					Usage:   "The path to the config file. ",
					Aliases: []string{"c"},
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
					Name:    "debug",
					Usage:   "Save additional build files for debugging. ",
					Aliases: []string{"d"},
				},
				&cli.StringSliceFlag{
					Name:    "clang-args",
					Aliases: []string{"a"},
					Usage: "Pass additional arguments to clang. " +
						"Useful for passing flags like -O2 or -g.",
				},
			},
			Action: run,
		},
	)
}

var outpath string

func build(c *cli.Context) error {
	isWindows := runtime.GOOS == "windows"

	if c.Bool("ebnf") {
		fmt.Println(parser.Parser().String())
		return nil
	}

	outpath = c.String("output")
	tmpDir, err := os.MkdirTemp("", "caffeinec")
	defer os.RemoveAll(tmpDir)
	if err != nil {
		return err
	}

	if outpath == "" {
		if isWindows {
			outpath = "output.exe"
		} else {
			outpath = "output"
		}
	}

	var conf project.CfConf

	f := c.Args().First()
	if f == "" {
		confPath := "." + string(filepath.Separator) + "cfconf.yaml"

		if c.String("config") != "" {
			confPath = c.String("config")
		}
		confPath = strings.TrimSuffix(confPath, "cfconf.yaml")

		conf, err = project.GetCfConf(confPath)
		if err != nil {
			return err
		}
		f = filepath.Join(confPath, conf.Main)
		outpath = filepath.Join(confPath, outpath)
	}

	pcache := cache.PackageCache{}
	pcache.Init()
	pcache.CacheScan(false)

	compiledCache := make(map[string]bool)

	imports, err := processIncludes(append([]string{f}, c.StringSlice("include")...), tmpDir, c.Int("opt-level"), c.Bool("debug"), c.Bool("header"), pcache, compiledCache)
	if err != nil {
		return err
	}

	if c.Bool("header") {
		cmd := exec.Command("sh", "-c", "mv "+tmpDir+"/*.h caffeine.h")
		err = cmd.Run()
		if err != nil {
			return err
		}
	}

	var stderr bytes.Buffer

	extra := []string{}
	if c.Bool("obj") {
		extra = append(extra, "-c")
	}

	args := append([]string{"clang"}, imports...)
	args = append(args, extra...)
	args = append(args, c.StringSlice("clang-args")...)

	if !c.Bool("obj") {
		args = append(args, "-o", outpath)
	}

	cmd := exec.Command(args[0], args[1:]...)

	cmd.Stderr = &stderr

	err = cmd.Run()
	var cerr error
	if err != nil {
		log.Println("stderr:", stderr.String())
		log.Println(err)
		cerr = err
	}

	if c.Bool("debug") {
		err = os.Mkdir("debug", 0755)
		if !os.IsExist(err) {
			return err
		} else if os.IsExist(err) {
			err = os.RemoveAll("debug")
			if err != nil {
				return err
			}
			err = os.Mkdir("debug", 0755)
			if err != nil {
				return err
			}
		}

		cmd := exec.Command("sh", "-c", "mv "+tmpDir+"/* debug")
		err = cmd.Run()
		if err != nil {
			return err
		}
	}

	return cerr
}

func run(c *cli.Context) error {
	err := build(c)
	if err != nil {
		return err
	}

	if !strings.Contains(outpath, "/") {
		outpath = "./" + outpath
	}

	cmd := exec.Command(outpath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	err = cmd.Run()
	if err != nil {
		return cli.Exit(color.RedString("Error running binary: %s", err), 1)
	}

	return nil
}

func parseAndCompile(path, tmpdir string, dump, header bool, pcache cache.PackageCache) (string, []string, error) {
	ast := parser.ParseFile(path)
	if dump {
		cwd, err := os.Getwd()
		if err != nil {
			return "", []string{}, cli.Exit(color.RedString("Error getting current working directory: %s", err), 1)
		}

		relativePath, err := filepath.Rel(cwd, path)
		if err != nil || filepath.IsAbs(relativePath) {
			relativePath = filepath.Base(path)
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

	comp := compiler.NewCompiler()
	comp.PackageCache = pcache
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

	if header {
		hf, err := os.CreateTemp(tmpdir, "caffeinec*.h")
		if err != nil {
			return "", []string{}, err
		}
		defer hf.Close()

		writeHeader(hf, comp)
	}

	return f.Name(), req, nil
}

func convertCffTypeToCType(t types.Type) string {
	switch typ := t.(type) {
	case *types.IntType:
		if typ.BitSize <= 8 {
			return "char"
		} else if typ.BitSize <= 16 {
			return "short"
		} else if typ.BitSize <= 32 {
			return "long"
		} else {
			return "long long"
		}
	case *types.FloatType:
		if typ.Kind == types.FloatKindFloat {
			return "float"
		} else if typ.Kind == types.FloatKindDouble {
			return "double"
		} else {
			return "long double"
		}
	case *types.PointerType:
		// Call the function recursively with the ElemType and append a star before it
		return convertCffTypeToCType(typ.ElemType) + " *"
	default:
		return "void"
	}
}

func writeHeader(f *os.File, comp *compiler.Compiler) error {
	_, err := f.WriteString("/*\n")
	if err != nil {
		return err
	}

	_, err = f.WriteString(" * This file was automatically generated by CaffeineC.\n")
	if err != nil {
		return err
	}

	_, err = f.WriteString(" * Do not edit this file directly.\n")
	if err != nil {
		return err
	}

	_, err = f.WriteString(" */\n")
	if err != nil {
		return err
	}

	_, err = f.WriteString("#ifndef CAFFEINEC_H\n")
	if err != nil {
		return err
	}

	_, err = f.WriteString("#define CAFFEINEC_H\n")
	if err != nil {
		return err
	}

	for _, fn := range comp.Module.Funcs {
		if strings.Count(fn.Name(), ".") > 0 {
			continue
		}
		_, err = f.WriteString(convertCffTypeToCType(fn.Sig.RetType) + " ")
		if err != nil {
			return err
		}

		_, err = f.WriteString(fn.Name())
		if err != nil {
			return err
		}

		_, err = f.WriteString("(")
		if err != nil {
			return err
		}

		for i, param := range fn.Sig.Params {
			_, err = f.WriteString(convertCffTypeToCType(param))
			if err != nil {
				return err
			}

			_, err = f.WriteString(" ")
			if err != nil {
				return err
			}

			_, err = f.WriteString(fn.Params[i].Name())
			if err != nil {
				return err
			}

			if i != len(fn.Sig.Params)-1 {
				_, err = f.WriteString(", ")
				if err != nil {
					return err
				}
			}
		}

		if fn.Sig.Variadic {
			_, err = f.WriteString(", ...")
			if err != nil {
				return err
			}
		}

		_, err = f.WriteString(")")
		if err != nil {
			return err
		}

		_, err = f.WriteString(";\n")
		if err != nil {
			return err
		}
	}

	for _, c := range comp.Module.TypeDefs {
		_, err = f.WriteString("class " + c.Name() + "\n{\nprivate:\n")
		if err != nil {
			return err
		}

		for _, field := range comp.StructFields[c.Name()] {
			if !field.Private {
				continue
			}
			_, err = f.WriteString(convertCffTypeToCType(comp.Context.StringToType(field.Type)) + " " + field.Name + ";\n")
			if err != nil {
				return err
			}
		}

		_, err = f.WriteString("public:\n")
		if err != nil {
			return err
		}

		for _, field := range comp.StructFields[c.Name()] {
			if field.Private {
				continue
			}
			_, err = f.WriteString(convertCffTypeToCType(comp.Context.StringToType(field.Type)) + " " + field.Name + ";\n")
			if err != nil {
				return err
			}
		}

		for _, fn := range comp.Module.Funcs {
			var parts []string
			if strings.Count(fn.Name(), ".") == 0 {
				continue
			} else {
				parts = strings.Split(fn.Name(), ".")
				if parts[0] != c.Name() {
					continue
				}
			}

			isConstructor := parts[1] == "constructor"

			if !isConstructor {
				_, err = f.WriteString(convertCffTypeToCType(fn.Sig.RetType) + " ")
				if err != nil {
					return err
				}

				_, err = f.WriteString(parts[1])
				if err != nil {
					return err
				}
			} else {
				_, err = f.WriteString(c.Name())
				if err != nil {
					return err
				}
			}

			_, err = f.WriteString("(")
			if err != nil {
				return err
			}

			for i, param := range fn.Sig.Params[1:] {
				_, err = f.WriteString(convertCffTypeToCType(param))
				if err != nil {
					return err
				}

				_, err = f.WriteString(" ")
				if err != nil {
					return err
				}

				_, err = f.WriteString(fn.Params[i+1].Name())
				if err != nil {
					return err
				}

				if i != len(fn.Sig.Params)-2 {
					_, err = f.WriteString(", ")
					if err != nil {
						return err
					}
				}
			}

			_, err = f.WriteString(")")
			if err != nil {
				return err
			}

			_, err = f.WriteString(";\n")
			if err != nil {
				return err
			}
		}

		_, err = f.WriteString("};\n")
		if err != nil {
			return err
		}
	}

	_, err = f.WriteString("#endif\n")
	if err != nil {
		return err
	}

	return nil
}

func processIncludes(includes []string, tmpDir string, opt int, dump, header bool, pcache cache.PackageCache, compiledCache map[string]bool) ([]string, error) {
	var files []string

	var wg sync.WaitGroup
	errs := make(chan error, 1)

	for _, include := range includes {
		info, err := os.Stat(include)
		if err != nil {
			return nil, err
		}
		if _, ok := compiledCache[include]; ok {
			continue
		}

		if info.IsDir() {
			err = filepath.Walk(include, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}

				if !info.IsDir() {
					if _, ok := compiledCache[path]; ok {
						return nil
					}
					err = processFile(path, &files, tmpDir, opt, dump, header, &wg, errs, pcache, compiledCache)
					if err != nil {
						return err
					}
				}

				return nil
			})
		} else {
			err = processFile(include, &files, tmpDir, opt, dump, header, &wg, errs, pcache, compiledCache)
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

func processFile(path string, files *[]string, tmpDir string, opt int, dump, header bool, wg *sync.WaitGroup, errs chan<- error, pcache cache.PackageCache, compiledCache map[string]bool) error {
	ext := filepath.Ext(path)
	if ext == ".c" || ext == ".cpp" || ext == ".h" || ext == ".o" {
		*files = append(*files, path)
	} else if ext == ".cffc" {

		wg.Add(1)
		go func(path string) {
			defer wg.Done()
			llFile, req, err := parseAndCompile(path, tmpDir, dump, header, pcache)
			if err != nil {
				errs <- err
				return
			}

			// Add the file to the compiled cache
			compiledCache[path] = true
			reqs := []string{}
			if len(req) > 0 {
				reqs, err = processIncludes(req, tmpDir, opt, false, header, pcache, compiledCache)
				if err != nil {
					errs <- err
					return
				}
			}

			*files = append(*files, llFile)
			for _, req := range reqs {
				if !slices.Contains(*files, req) {
					*files = append(*files, req)
				}
			}
		}(path)
	}

	return nil
}
