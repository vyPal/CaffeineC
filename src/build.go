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
			&cli.StringSliceFlag{
				Name:    "llc-args",
				Aliases: []string{"l"},
				Usage: "Pass additional arguments to llc. " +
					"Useful for passing flags like -O2 or -g.",
			},
			&cli.StringSliceFlag{
				Name:    "gcc-args",
				Aliases: []string{"g"},
				Usage: "Pass additional arguments to gcc. " +
					"Useful for passing flags like -O2 or -g.",
			},
			&cli.BoolFlag{
				Name: "use-gcc",
				Usage: "Use gcc instead of clang for linking. " +
					"Useful for linking with C++ code. ",
				Aliases: []string{"G"},
			},
			&cli.BoolFlag{
				Name: "use-tcc",
				Usage: "Use tcc instead of clang for linking. " +
					"Useful for linking with C code. ",
				Aliases: []string{"T"},
			},
			&cli.BoolFlag{
				Name:    "no-cache",
				Aliases: []string{"n"},
				Usage:   "Disables caching",
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
				&cli.StringSliceFlag{
					Name:    "llc-args",
					Aliases: []string{"l"},
					Usage: "Pass additional arguments to llc. " +
						"Useful for passing flags like -O2 or -g.",
				},
				&cli.StringSliceFlag{
					Name:    "gcc-args",
					Aliases: []string{"g"},
					Usage: "Pass additional arguments to gcc. " +
						"Useful for passing flags like -O2 or -g.",
				},
				&cli.BoolFlag{
					Name: "use-gcc",
					Usage: "Use gcc instead of clang for linking. " +
						"Useful for linking with C++ code. ",
					Aliases: []string{"G"},
				},
				&cli.BoolFlag{
					Name: "use-tcc",
					Usage: "Use tcc instead of clang for linking. " +
						"Useful for linking with C code. ",
					Aliases: []string{"T"},
				},
				&cli.BoolFlag{
					Name:    "no-cache",
					Aliases: []string{"n"},
					Usage:   "Disables caching",
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
var precompiledCache map[string]string
var cwd string
var builtFiles []cache.BuiltFile

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

	cwd, err = os.Getwd()
	if err != nil {
		return cli.Exit(color.RedString("Error getting current working directory: %s", err), 1)
	}

	var conf project.CfConf

	f := c.Args().First()
	if f == "" {
		confPath := cwd + string(filepath.Separator) + "cfconf.yaml"

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
	precompiledCache = make(map[string]string)
	header = c.Bool("header")
	debug = c.Bool("debug")

	builtFiles = []cache.BuiltFile{}

	proj, err := pcache.CreateProject(cwd)
	if err != nil {
		return err
	}

	ic := []string{}
	if !c.Bool("no-cache") {
		ic, err = proj.SumDiff()
		if err != nil {
			return err
		}

		abf, err := proj.GetBuiltFiles(ic)
		if err != nil {
			return err
		}
		builtFiles = abf

		for _, cached := range abf {
			precompiledCache[cached.FilePath] = cached.ObjPath
		}
	}

	llFiles, imports, err := processIncludes(append([]string{f}, c.StringSlice("include")...))
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

	if c.Bool("use-gcc") {
		for _, file := range llFiles {
			llcArgs := append([]string{"-filetype=obj", "-o", strings.TrimSuffix(file, ".ll") + ".o", file}, c.StringSlice("llc-args")...)
			llcCmd := exec.Command("llc", llcArgs...)
			llcCmd.Stderr = &stderr

			err = llcCmd.Run()
			if err != nil {
				log.Println("stderr:", stderr.String())
				log.Println(err)
				return err
			}
			imports = append(imports, strings.TrimSuffix(file, ".ll")+".o")
			if !c.Bool("no-cache") {
				rp, err := filepath.Rel(tmpDir, file)
				if err != nil {
					log.Println(err)
				}
				builtFiles = append(builtFiles, cache.BuiltFile{FilePath: string(filepath.Separator) + rp, ObjPath: strings.TrimSuffix(file, ".ll") + ".o"})
			}
		}

		gccArgs := append([]string{"-o", outpath}, imports...)
		gccArgs = append(gccArgs, extra...)
		gccArgs = append(gccArgs, c.StringSlice("gcc-args")...)

		if !c.Bool("obj") {
			gccArgs = append(gccArgs, "-o", outpath)
		}

		var gccCmd *exec.Cmd

		if c.Bool("use-tcc") {
			gccCmd = exec.Command("tcc", gccArgs...)
		} else {
			gccCmd = exec.Command("gcc", gccArgs...)
		}

		gccCmd.Stderr = &stderr

		err = gccCmd.Run()
		if err != nil {
			log.Println("stderr:", stderr.String())
			log.Println(err)
		}
	} else {
		if !c.Bool("no-cache") {
			for _, file := range llFiles {
				args := append([]string{"clang", file}, "-c")
				args = append(args, []string{"-o", strings.TrimSuffix(file, ".ll") + ".o"}...)
				args = append(args, c.StringSlice("clang-args")...)

				cmd := exec.Command(args[0], args[1:]...)
				cmd.Stderr = &stderr
				cmd.Dir = tmpDir

				err = cmd.Run()
				if err != nil {
					log.Println("stderr:", stderr.String())
					log.Println(err)
				}
				rp, err := filepath.Rel(tmpDir, file)
				if err != nil {
					log.Println(err)
				}
				builtFiles = append(builtFiles, cache.BuiltFile{FilePath: string(filepath.Separator) + rp, ObjPath: strings.TrimSuffix(file, ".ll") + ".o"})
			}
		}

		args := append([]string{"clang"}, imports...)
		args = append(args, llFiles...)
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
	}

	if !c.Bool("no-cache") {
		err = proj.SaveBuiltFiles(builtFiles, ic)
		if err != nil {
			return err
		}
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

		cmd := exec.Command("sh", "-c", "mv "+tmpDir+"/* debug/")
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

	cmd := exec.Command(outpath, c.Args().Tail()...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	err = cmd.Run()
	if err != nil {
		return cli.Exit(color.RedString("Error running binary: %s", err), 1)
	}

	return nil
}

func parseAndCompile(path string, wg *sync.WaitGroup, errs chan<- error, files *[]string, llfiles *[]string) (string, error) {
	ast := parser.ParseFile(path)
	if debug {
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
	if precompiledCache[path] == "" {
		err = comp.FindImports()
		if err != nil {
			return "", err
		}
		reqs := comp.RequiredImports
		for _, req := range reqs {
			if compiledCache[req] {
				continue
			}
			err := processFile(req, files, llfiles, wg, errs)
			if err != nil {
				return "", err
			}
		}
		err = comp.Compile()
		if err != nil {
			return "", cli.Exit(color.RedString("Error compiling: %s", err), 1)
		}

		err := os.MkdirAll(filepath.Dir(filepath.Join(tmpDir, strings.TrimSuffix(path, ".cffc")+".ll")), 0755)
		if err != nil {
			return "", err
		}

		f, err := os.Create(filepath.Join(tmpDir, strings.TrimSuffix(path, ".cffc")+".ll"))
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
	} else {
		*files = append(*files, precompiledCache[path])
		reqs, err := comp.ListImportedFiles()
		if err != nil {
			return "", err
		}
		for _, req := range reqs {
			if compiledCache[req] {
				continue
			}
			err := processFile(req, files, llfiles, wg, errs)
			if err != nil {
				return "", err
			}
		}
	}

	return "", nil
}

func processIncludes(includes []string) ([]string, []string, error) {
	var files []string
	var llfiles []string

	var wg sync.WaitGroup
	errs := make(chan error, 1)

	for _, include := range includes {
		err := processFile(include, &files, &llfiles, &wg, errs)
		if err != nil {
			return nil, nil, err
		}
	}

	go func() {
		wg.Wait()
		close(errs)
	}()

	if err, ok := <-errs; ok {
		return nil, nil, err
	}

	return llfiles, files, nil
}

func processFile(path string, files *[]string, llfiles *[]string, wg *sync.WaitGroup, errs chan<- error) error {
	if _, ok := compiledCache[path]; ok {
		return nil
	}
	ext := filepath.Ext(path)
	if ext == ".c" || ext == ".cpp" || ext == ".h" || ext == ".o" || ext == ".a" || ext == ".so" || ext == ".dll" || ext == ".dylib" || ext == ".ll" {
		*files = append(*files, path)
		compiledCache[path] = true
	} else if ext == ".cffc" {
		compiledCache[path] = true
		wg.Add(1)
		go func(path string) {
			defer wg.Done()
			llFile, err := parseAndCompile(path, wg, errs, files, llfiles)
			if err != nil {
				errs <- err
				return
			}
			if llFile != "" {
				*llfiles = append(*llfiles, llFile)
			}
		}(path)
	} else if ext == "" {
		dir, err := os.ReadDir(path)
		if err != nil {
			return err
		}

		for _, entry := range dir {
			err := processFile(filepath.Join(path, entry.Name()), files, llfiles, wg, errs)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
