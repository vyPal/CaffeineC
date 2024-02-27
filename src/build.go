package main

import (
	"bytes"
	"encoding/json"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/fatih/color"
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

var err error
var tmpDir string
var debug bool
var header bool
var pcache cache.PackageCache
var compiledCache map[string]bool

func build(c *cli.Context) error {
	outpath = c.String("output")
	tmpDir, err = os.MkdirTemp("", "caffeinec")
	defer os.RemoveAll(tmpDir)
	if err != nil {
		return err
	}

	if outpath == "" {
		if runtime.GOOS == "windows" {
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

	pcache = cache.PackageCache{}
	pcache.Init()
	pcache.CacheScan(false)

	compiledCache = make(map[string]bool)
	header = c.Bool("header")
	debug = c.Bool("debug")

	imports, err := processIncludes(append([]string{f}, c.StringSlice("include")...))
	if err != nil {
		return err
	}

	if header {
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
	if err != nil {
		log.Println("stderr:", stderr.String())
		log.Println(err)
	}

	if debug {
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

		cmd = exec.Command("sh", "-c", "mv "+tmpDir+"/* debug/")
		err = cmd.Run()
		if err != nil {
			return err
		}
	}

	return err
}

func run(c *cli.Context) error {
	err := build(c)
	if err != nil {
		return err
	}

	if !strings.Contains(outpath, "/") {
		outpath = "./" + outpath
	}

	cmd := exec.Command(outpath, c.Args().Slice()...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	err = cmd.Run()
	if err != nil {
		return cli.Exit(color.RedString("Error running binary: %s", err), 1)
	}

	return nil
}

func parseAndCompile(path string, wg *sync.WaitGroup, errs chan<- error, files *[]string) (string, error) {
	ast := parser.ParseFile(path)
	if debug {
		cwd, err := os.Getwd()
		if err != nil {
			return "", cli.Exit(color.RedString("Error getting current working directory: %s", err), 1)
		}

		relativePath, err := filepath.Rel(cwd, path)
		if err != nil || filepath.IsAbs(relativePath) {
			relativePath = filepath.Base(path)
		}

		fullPath := filepath.Join(tmpDir, "ast-"+relativePath+".json")
		dirPath := filepath.Dir(fullPath)

		err = os.MkdirAll(dirPath, 0755)
		if err != nil {
			return "", cli.Exit(color.RedString("Error creating directories: %s", err), 1)
		}

		astFile, err := os.Create(fullPath)
		if err != nil {
			return "", cli.Exit(color.RedString("Error creating AST dump file: %s", err), 1)
		}
		defer astFile.Close()

		encoder := json.NewEncoder(astFile)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(ast); err != nil {
			return "", cli.Exit(color.RedString("Error encoding AST: %s", err), 1)
		}
	}

	comp := compiler.NewCompiler()
	comp.PackageCache = pcache
	wDir, err := filepath.Abs(filepath.Dir(path))
	if err != nil {
		return "", err
	}
	comp.Init(ast, wDir)
	err = comp.FindImports()
	if err != nil {
		return "", err
	}
	reqs := comp.RequiredImports
	for _, req := range reqs {
		if compiledCache[req] {
			continue
		}
		err := processFile(req, files, wg, errs)
		if err != nil {
			return "", err
		}
	}
	err = comp.Compile()
	if err != nil {
		return "", cli.Exit(color.RedString("Error compiling: %s", err), 1)
	}

	f, err := os.CreateTemp(tmpDir, "caffeinec*.ll")
	if err != nil {
		return "", err
	}
	defer f.Close()

	_, err = f.WriteString(comp.Module.String())
	if err != nil {
		return "", err
	}

	if header {
		hf, err := os.CreateTemp(tmpDir, "caffeinec*.h")
		if err != nil {
			return "", err
		}
		defer hf.Close()

		compiler.WriteHeader(hf, comp)
	}

	return f.Name(), nil
}

func processIncludes(includes []string) ([]string, error) {
	var files []string

	var wg sync.WaitGroup
	errs := make(chan error, 1)

	for _, include := range includes {
		err := processFile(include, &files, &wg, errs)
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

func processFile(path string, files *[]string, wg *sync.WaitGroup, errs chan<- error) error {
	if _, ok := compiledCache[path]; ok {
		return nil
	}
	ext := filepath.Ext(path)
	if ext == ".c" || ext == ".cpp" || ext == ".h" || ext == ".o" || ext == ".a" || ext == ".so" || ext == ".dll" || ext == ".dylib" || ext == ".ll" {
		*files = append(*files, path)
	} else if ext == ".cffc" {
		compiledCache[path] = true
		wg.Add(1)
		go func(path string) {
			defer wg.Done()
			llFile, err := parseAndCompile(path, wg, errs, files)
			if err != nil {
				errs <- err
				return
			}
			*files = append(*files, llFile)
		}(path)
	}

	return nil
}
