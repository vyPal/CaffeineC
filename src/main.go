package main

import (
	"bufio"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/fatih/color"
	"github.com/urfave/cli/v2"
	"github.com/vyPal/CaffeineC/lib/compiler"
	"github.com/vyPal/CaffeineC/lib/parser"
)

func main() {
	app := &cli.App{
		Name:                   "CaffeineC",
		Usage:                  "A C-like language that compiles to LLVM IR",
		EnableBashCompletion:   true,
		Suggest:                true,
		UseShortOptionHandling: true,
		Version:                "2.0.2",
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
			{
				Name:  "autocomplete",
				Usage: "Install autocomplete script for CaffeineC",
				Action: func(c *cli.Context) error {
					shell := filepath.Base(os.Getenv("SHELL"))
					homeDir, _ := os.UserHomeDir()
					var autocompleteScriptURL, shellConfigFile string

					switch shell {
					case "bash":
						autocompleteScriptURL = "https://raw.githubusercontent.com/vyPal/CaffeineC/master/autocomplete/bash_autocomplete"
						shellConfigFile = filepath.Join(homeDir, ".bashrc")
					case "zsh":
						autocompleteScriptURL = "https://raw.githubusercontent.com/vyPal/CaffeineC/master/autocomplete/zsh_autocomplete"
						shellConfigFile = filepath.Join(homeDir, ".zshrc")
					default:
						fmt.Println("Unsupported shell for autocomplete. Skipping...")
						return nil
					}

					installDir := path.Join(homeDir, ".local", "share", "CaffeineC")
					if err := os.MkdirAll(installDir, 0755); err != nil {
						return err
					}
					autocompleteScriptPath := filepath.Join(installDir, "CaffeineC_autocomplete")

					fmt.Printf("Downloading autocomplete script for %s...\n", shell)
					err := downloadFile(autocompleteScriptURL, autocompleteScriptPath)
					if err != nil {
						return err
					}

					// Add the source command to the shell's configuration file to make it persistent
					file, err := os.OpenFile(shellConfigFile, os.O_RDWR, 0644)
					if err != nil {
						return err
					}
					defer file.Close()

					// Check if the source line already exists in the file
					sourceLine := fmt.Sprintf("source %s", autocompleteScriptPath)
					scanner := bufio.NewScanner(file)
					for scanner.Scan() {
						if strings.Contains(scanner.Text(), sourceLine) {
							fmt.Println("Autocomplete script already installed.")
							return nil
						}
					}

					// If the source line doesn't exist, append it to the file
					_, err = file.WriteString(sourceLine)
					if err != nil {
						return err
					}

					fmt.Println("Autocomplete script installed. It will be sourced automatically in new shell sessions.")
					fmt.Println("To source it in the current session, run:")
					relativePath := strings.Replace(autocompleteScriptPath, homeDir, "~", 1)
					fmt.Printf("\tsource %s\n", relativePath)
					return nil
				},
			},
			{
				Name:  "update",
				Usage: "Update CaffeineC to the latest version",
				Action: func(c *cli.Context) error {
					resp, err := http.Get("https://api.github.com/repos/vyPal/CaffeineC/releases/latest")
					if err != nil {
						fmt.Println("Failed to fetch the latest release:", err)
						return nil
					}
					defer resp.Body.Close()

					var release Release
					if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
						fmt.Println("Failed to decode the release data:", err)
						return nil
					}

					// Remove the 'v' prefix from the tag name
					latestVersion := strings.TrimPrefix(release.TagName, "v")

					if latestVersion != c.App.Version {
						fmt.Printf("A new version is available: %s. Updating...\n", latestVersion)

						// Download the new binary
						resp, err = http.Get("https://github.com/vyPal/CaffeineC/releases/download/" + release.TagName + "/CaffeineC")
						if err != nil {
							fmt.Println("Failed to download the new version:", err)
							return nil
						}
						defer resp.Body.Close()

						// Write the new binary to a temporary file
						tmpFile, err := os.CreateTemp("", "CaffeineC")
						if err != nil {
							fmt.Println("Failed to create a temporary file:", err)
							return nil
						}
						defer os.Remove(tmpFile.Name())

						_, err = io.Copy(tmpFile, resp.Body)
						if err != nil {
							fmt.Println("Failed to write to the temporary file:", err)
							return nil
						}

						// Expand the ~ to the user's home directory
						homeDir, err := os.UserHomeDir()
						if err != nil {
							fmt.Println("Failed to get the user's home directory:", err)
							return nil
						}

						// Set the destination file path
						dstFilePath := filepath.Join(homeDir, ".local", "bin", "CaffeineC")

						// Create the destination file
						dstFile, err := os.Create(dstFilePath)
						if err != nil {
							fmt.Println("Failed to create the destination file:", err)
							return nil
						}
						defer dstFile.Close()

						// Copy the temporary file to the destination file
						_, err = io.Copy(dstFile, tmpFile)
						if err != nil {
							fmt.Println("Failed to copy the file:", err)
							return nil
						}

						// Make the temporary file executable
						if err := os.Chmod(dstFilePath, 0755); err != nil {
							fmt.Println("Failed to make the temporary file executable:", err)
							return nil
						}

						// Remove the temporary file
						if err := os.Remove(tmpFile.Name()); err != nil {
							fmt.Println("Failed to remove the temporary file:", err)
							return nil
						}

						fmt.Println("Update successful!")
					} else {
						fmt.Println("You're up to date!")
					}

					return nil
				},
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		panic(err)
	}
}

func downloadFile(url string, filepath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

type Release struct {
	TagName string `json:"tag_name"`
}

func checkUpdate(c *cli.Context) {
	resp, err := http.Get("https://api.github.com/repos/vyPal/CaffeineC/releases/latest")
	if err != nil {
		fmt.Println("Failed to fetch the latest release:", err)
		return
	}
	defer resp.Body.Close()

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		fmt.Println("Failed to decode the release data:", err)
		return
	}

	// Remove the 'v' prefix from the tag name
	latestVersion := strings.TrimPrefix(release.TagName, "v")

	if latestVersion != c.App.Version {
		fmt.Printf("A new version is available: %s to update, run 'CaffeineC update'\n", latestVersion)
	} else {
		fmt.Println("You're up to date!")
	}
}

func build(c *cli.Context) error {
	checkUpdate(c)
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
	cmd.Stdin = os.Stdin
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
