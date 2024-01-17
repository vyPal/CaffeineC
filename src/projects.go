package main

import (
	"bufio"
	"encoding/gob"
	"fmt"
	"log"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/fatih/color"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/urfave/cli/v2"
	"github.com/vyPal/CaffeineC/util"
	"gopkg.in/yaml.v3"
)

func init() {
	commands = append(commands, &cli.Command{
		Name:     "init",
		Usage:    "Initialize a new CaffeineC project",
		Category: "project",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "name",
				Aliases: []string{"n"},
				Usage:   "The name of the project",
			},
			&cli.StringFlag{
				Name:    "version",
				Aliases: []string{"v"},
				Usage:   "The version of the project",
			},
			&cli.StringFlag{
				Name:    "main",
				Aliases: []string{"m"},
				Usage:   "The main file of the project",
			},
			&cli.StringFlag{
				Name:    "author",
				Aliases: []string{"a"},
				Usage:   "The author of the project",
			},
			&cli.StringFlag{
				Name:    "license",
				Aliases: []string{"l"},
				Usage:   "The license of the project",
			},
		},
		Action: initProject,
	}, &cli.Command{
		Name:    "install",
		Aliases: []string{"i"},
		Usage:   "Installs a package and adds it to the project",
		Description: "Clones a package from a remote git repository to the package cache." +
			"\nIf the package is already in the cache, it only adds a reference to it to the project's config file." +
			"\n\nIf the command is run without the url argument, it installs all package fro the current project.",
		Category:  "project",
		ArgsUsage: "<url>",
		Action:    install,
	}, &cli.Command{
		Name:    "library",
		Aliases: []string{"lib"},
		Usage:   "Manages theCaffeineC libraries",
		Subcommands: []*cli.Command{
			{
				Name:  "info",
				Usage: "Displays information about a package",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "version",
						Aliases: []string{"v"},
						Usage:   "The version of the package",
					},
				},
				Action: libInfo,
			},
		},
		Category: "project",
		Action:   libInfo,
	})
}

type CfConf struct {
	Name         string             `yaml:"name"`
	Description  string             `yaml:"description"`
	Version      string             `yaml:"version"`
	Main         string             `yaml:"main"`
	Dependencies []CFConfDependency `yaml:"dependencies"`
	Author       string             `yaml:"author"`
	License      string             `yaml:"license"`
}

type CFConfDependency struct {
	Package    string `yaml:"package"`
	Version    string `yaml:"version"`
	Identifier string `yaml:"identifier"`
}

func (c *CfConf) CreateDefault() {
	c.Name = "NewProject"
	c.Description = "A new CaffeineC project"
	c.Version = "1.0.0"
	c.Main = "src/main.cffc"
	c.Author = "Anonymous"
	c.License = "MIT"
}

func (c *CfConf) Save(filepath string, overwrite bool) error {
	if _, err := os.Stat(filepath); !os.IsNotExist(err) {
		if overwrite || promptYN(filepath+" already exists. Overwrite?", false) {
			os.Remove(filepath)
		} else {
			return nil
		}
	}

	_, err := os.Create(filepath)
	if err != nil {
		return err
	}

	file, err := os.OpenFile(filepath, os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	yml, err := yaml.Marshal(c)
	if err != nil {
		return err
	}

	_, err = file.Write(yml)
	if err != nil {
		return err
	}

	return nil
}

func install(c *cli.Context) error {
	liburl := c.Args().First()
	conf, err := GetCfConf("")
	if err != nil {
		return err
	}

	cache := PackageCache{}
	err = cache.Init()
	if err != nil {
		return err
	}

	if liburl == "" {
		for _, dep := range conf.Dependencies {
			pkg, err := cache.GetPackage(dep.Package, dep.Version, dep.Identifier)
			if err != nil {
				return err
			}

			if pkg == (Package{}) {
				color.Green("Package not found locally, cloning...")
				conf, _, _, err = InstallLibrary(cache, dep.Identifier)
				if err != nil {
					return err
				}
			} else {
				conf, err = GetCfConf(pkg.Path)
				if err != nil {
					return err
				}
			}
		}
	} else {
		err = cache.CacheScan(true)
		if err != nil {
			return err
		}

		liburl, version, err := PrepUrl(liburl)
		if err != nil {
			return err
		}

		pkg, err := cache.GetPackage("", "", filepath.Join(strings.TrimPrefix(liburl, "https://"), version))
		if err != nil {
			return err
		}

		var ident, ver string
		if pkg == (Package{}) {
			color.Green("Package not found locally, cloning...")
			conf, ident, ver, err = InstallLibrary(cache, liburl)
			if err != nil {
				return err
			}
		} else {
			ident = pkg.Identifier
			ver = pkg.Version
			conf, err = GetCfConf(pkg.Path)
			if err != nil {
				return err
			}
		}

		dep := CFConfDependency{
			Package:    conf.Name,
			Version:    ver,
			Identifier: ident,
		}

		conf.Dependencies = append(conf.Dependencies, dep)
		err = conf.Save("cfconf.yaml", true)
		if err != nil {
			return err
		}

		fmt.Println("Added package", conf.Name, "to the project.")
	}

	fmt.Println("--------------------------------------------------")
	fmt.Println("                  Package Details                 ")
	fmt.Println("--------------------------------------------------")
	fmt.Printf("Name        : %s\n", conf.Name)
	fmt.Printf("Description : %s\n", conf.Description)
	fmt.Printf("Version     : %s\n", conf.Version)
	fmt.Printf("Main File   : %s\n", conf.Main)
	fmt.Printf("Author      : %s\n", conf.Author)
	fmt.Printf("License     : %s\n", conf.License)
	fmt.Println("--------------------------------------------------")

	return nil
}

func PrepUrl(liburl string) (u, ver string, e error) {
	version := "main"
	if strings.Contains(liburl, "@") {
		split := strings.Split(liburl, "@")
		liburl = split[0]
		version = split[1]
	} else {
		color.Yellow("Branch name not specified, defaulting to 'main'")
	}

	parsedUrl, err := url.Parse(liburl)
	if err != nil {
		return "", "", err
	}

	if parsedUrl.Hostname() == "" {
		liburl = "https://github.com/" + liburl
	}

	if !strings.HasPrefix(liburl, "http://") && !strings.HasPrefix(liburl, "https://") {
		liburl = "https://" + liburl
	}
	return liburl, version, nil
}

func InstallLibrary(cache PackageCache, liburl string) (conf CfConf, ident, ver string, e error) {
	liburl, version, err := PrepUrl(liburl)
	if err != nil {
		return CfConf{}, "", "", err
	}

	// Create a directory in the cache's BaseDir
	installDir := filepath.Join(cache.BaseDir, strings.TrimPrefix(liburl, "https://"), version)
	err = os.MkdirAll(installDir, 0700)
	if err != nil {
		return CfConf{}, "", "", err
	}

	// Clone the repository to the install directory
	_, err = git.PlainClone(installDir, false, &git.CloneOptions{
		URL:           liburl,
		SingleBranch:  true,
		Depth:         1,
		ReferenceName: plumbing.NewBranchReferenceName(version),
	})
	if err != nil {
		return CfConf{}, "", "", err
	}

	pkg := Package{
		Name:       conf.Name,
		Version:    conf.Version,
		Identifier: strings.TrimPrefix(liburl, "https://"),
		Path:       installDir,
	}
	cache.PkgList = append(cache.PkgList, pkg)
	err = cache.CacheSave()
	if err != nil {
		return CfConf{}, "", "", err
	}

	// Get the configuration file from the cloned repository
	conf, err = GetCfConf(installDir)
	if err != nil {
		return CfConf{}, "", "", err
	}

	return conf, strings.TrimPrefix(liburl, "https://"), version, nil
}

func libInfo(c *cli.Context) error {
	liburl := c.Args().First()
	var conf CfConf
	var err error
	if liburl == "" {
		conf, err = GetCfConf("")
		if err != nil {
			return err
		}
	} else {
		cache := PackageCache{}
		err = cache.Init()
		if err != nil {
			return err
		}

		err = cache.CacheScan(true)
		if err != nil {
			return err
		}

		pkg, err := cache.GetPackage("", "", liburl)
		if err != nil {
			return err
		}

		if pkg == (Package{}) {
			color.Green("Package not found locally, cloning...")
			liburl, version, err := PrepUrl(liburl)
			if err != nil {
				return err
			}

			// Create a temporary directory
			tempDir, err := os.MkdirTemp("", "cclib*")
			if err != nil {
				return err
			}
			defer os.RemoveAll(tempDir) // clean up

			// Clone the repository to the temporary directory
			_, err = git.PlainClone(tempDir, false, &git.CloneOptions{
				URL:           liburl,
				SingleBranch:  true,
				Depth:         1,
				ReferenceName: plumbing.NewBranchReferenceName(version),
			})
			if err != nil {
				return err
			}

			// Get the configuration file from the cloned repository
			conf, err = GetCfConf(tempDir)
			if err != nil {
				return err
			}
		} else {
			conf, err = GetCfConf(pkg.Path)
			if err != nil {
				return err
			}
		}
	}

	fmt.Println("--------------------------------------------------")
	fmt.Println("                  Package Details                 ")
	fmt.Println("--------------------------------------------------")
	fmt.Printf("Name        : %s\n", conf.Name)
	fmt.Printf("Description : %s\n", conf.Description)
	fmt.Printf("Version     : %s\n", conf.Version)
	fmt.Printf("Main File   : %s\n", conf.Main)
	fmt.Printf("Author      : %s\n", conf.Author)
	fmt.Printf("License     : %s\n", conf.License)
	fmt.Println("--------------------------------------------------")

	return nil
}

type PackageCache struct {
	BaseDir string
	RootDir string
	PkgList []Package
}

type Package struct {
	Name       string
	Version    string
	Identifier string
	Path       string
}

func (p *PackageCache) Init() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}

	libDir := path.Join(homeDir, ".local", "lib", "CaffeineC")
	if runtime.GOOS == "windows" {
		libDir = path.Join(homeDir, "AppData", "Local", "Programs", "CaffeineC")
	}

	err = os.MkdirAll(libDir, 0700)
	if err != nil {
		return err
	}

	cacheDir := path.Join(libDir, "packages")
	err = os.Mkdir(cacheDir, 0700)
	if err != nil && !os.IsExist(err) {
		return err
	}

	p.RootDir = libDir
	p.BaseDir = cacheDir
	p.PkgList = make([]Package, 0)

	return nil
}

func (p *PackageCache) DeepCacheScan() error {
	err := filepath.WalkDir(p.BaseDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		if d.IsDir() {
			conf, err := GetCfConf(path)
			if err != nil {
				if os.IsNotExist(err) {
					return nil
				} else {
					return err
				}
			}

			identifier := strings.TrimPrefix(path, p.BaseDir)
			identifier = strings.TrimPrefix(identifier, "/")

			pkg := Package{
				Name:       conf.Name,
				Version:    conf.Version,
				Identifier: identifier,
				Path:       path,
			}

			p.PkgList = append(p.PkgList, pkg)
			return filepath.SkipDir
		}

		return nil
	})

	if err != nil {
		return err
	}

	return nil
}

func (p *PackageCache) CacheScan(deepOnFail bool) error {
	cacheFile, err := os.Open(path.Join(p.BaseDir, "cache.bin"))
	if err != nil {
		if os.IsNotExist(err) {
			if deepOnFail {
				fmt.Println("Cache file not found, performing deep scan...")
				err := p.DeepCacheScan()
				if err != nil {
					return err
				}

				err = p.CacheSave()
				if err != nil {
					return err
				}
				return nil
			} else {
				return err
			}
		} else {
			return err
		}
	}

	decoder := gob.NewDecoder(cacheFile)
	err = decoder.Decode(&p.PkgList)
	if err != nil {
		return err
	}

	return nil
}

func (p *PackageCache) CacheSave() error {
	cacheFile, err := os.Create(path.Join(p.BaseDir, "cache.bin"))
	if err != nil {
		return err
	}

	encoder := gob.NewEncoder(cacheFile)
	err = encoder.Encode(p.PkgList)
	if err != nil {
		return err
	}

	return nil
}

func (p *PackageCache) GetPackage(name, version, identifier string) (Package, error) {
	for _, pkg := range p.PkgList {
		fmt.Printf("Name: %s, Version: %s, Ident: %s, SearchIdent: %s\n", pkg.Name, pkg.Version, pkg.Identifier, identifier)
		if (pkg.Name == name || name == "") && pkg.Identifier == identifier {
			if version == "" || version == "*" || version == pkg.Version {
				return pkg, nil
			}
			continue
		}
	}

	return Package{}, nil
}

func (p *PackageCache) HasPackage(name, version, identifier string) (bool, error) {
	for _, pkg := range p.PkgList {
		if pkg.Name == name && pkg.Identifier == identifier {
			if version == "" || version == "*" {
				return true, nil
			}

			sver, err := util.Parse(pkg.Version)
			if err != nil {
				return false, err
			}

			sat, err := sver.Satisfies(version)
			if err != nil {
				return false, err
			}

			if sat {
				return true, nil
			}
			continue
		}
	}

	return false, nil
}

func initProject(c *cli.Context) error {
	rootDir := c.Args().First()
	if rootDir == "" {
		rootDir = "."
	}

	// Check if the directory exists
	if _, err := os.Stat(rootDir); !os.IsNotExist(err) {
		// The directory exists, check if it has any contents
		files, err := os.ReadDir(rootDir)
		if err != nil {
			return err
		}

		if len(files) > 0 {
			if !promptYN("The directory is not empty, continue?", false) {
				return nil
			}
		}
	} else {
		err := os.Mkdir(rootDir, 0755)
		if err != nil {
			return err
		}

		fmt.Println("Created directory:", rootDir)
	}

	if _, err := os.Stat(path.Join(rootDir, "src")); os.IsNotExist(err) {
		err := os.Mkdir(path.Join(rootDir, "src"), 0755)
		if err != nil {
			return err
		}

		fmt.Println("Created directory:", path.Join(rootDir, "src"))
	}

	if _, err := os.Stat(path.Join(rootDir, "src", "main.cffc")); os.IsNotExist(err) {
		_, err := os.Create(path.Join(rootDir, "src", "main.cffc"))
		if err != nil {
			return err
		}

		file, err := os.OpenFile(path.Join(rootDir, "src", "main.cffc"), os.O_WRONLY, 0644)
		if err != nil {
			return err
		}

		_, err = file.WriteString("package main;\n\nextern func printf(format: *i8): void;\n\nfunc main(): i64 {\n\tprintf(\"Hello, world!\\n\");\n\treturn 0;\n}\n")
		if err != nil {
			return err
		}

		fmt.Println("Created file:", path.Join(rootDir, "src", "main.cffc"))
	}

	if promptYN("Use default configuration?", false) {
		conf := CfConf{}
		conf.CreateDefault()

		err := conf.Save(path.Join(rootDir, "cfconf.yaml"), false)
		if err != nil {
			return err
		}

		fmt.Println("Created file:", path.Join(rootDir, "cfconfig.yaml"))
	} else {
		conf := CfConf{}

		conf.Name = promptString("Project name", "NewProject")
		conf.Description = promptString("Project description", "A new CaffeineC project")
		conf.Version = promptString("Project version", "1.0.0")
		conf.Main = promptString("Main file", "src/main.cffc")
		conf.Author = promptString("Author", "Anonymous")
		conf.License = promptString("License", "MIT")

		err := conf.Save(path.Join(rootDir, "cfconf.yaml"), false)
		if err != nil {
			return err
		}

		fmt.Println("Created file:", path.Join(rootDir, "cfconf.yaml"))
	}

	fmt.Println("----------------------------------------")
	fmt.Println("Project initialized successfully!")
	fmt.Println("Run 'cd", rootDir, "&& caffeinec build' to build the project.'")
	fmt.Println("----------------------------------------")

	return nil
}

func GetCfConf(dir string) (CfConf, error) {
	var conf CfConf

	file, err := os.Open(path.Join(dir, "cfconf.yaml"))
	if err != nil {
		return CfConf{}, err
	}

	decoder := yaml.NewDecoder(file)
	err = decoder.Decode(&conf)
	if err != nil {
		return CfConf{}, err
	}

	return conf, nil
}

func promptYN(prompt string, def bool) bool {
	reader := bufio.NewReader(os.Stdin)

	if def {
		fmt.Printf("%s (Y/n): ", prompt)
	} else {
		fmt.Printf("%s (y/N): ", prompt)
	}

	response, err := reader.ReadString('\n')
	if err != nil {
		panic(err)
	}

	response = strings.TrimSpace(response)

	if response == "" {
		return def
	}

	return strings.ToLower(response) == "y"
}

func promptString(prompt string, def string) string {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("%s (%s): ", prompt, def)

	response, err := reader.ReadString('\n')
	if err != nil {
		panic(err)
	}

	response = strings.TrimSpace(response)

	if response == "" {
		return def
	}

	return response
}
