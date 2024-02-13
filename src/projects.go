package main

import (
	"fmt"
	"net/url"
	"os"
	"path"
	"strings"

	"github.com/fatih/color"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/urfave/cli/v2"
	"github.com/vyPal/CaffeineC/lib/cache"
	"github.com/vyPal/CaffeineC/lib/project"
	"github.com/vyPal/CaffeineC/util"
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
			{
				Name:   "update",
				Usage:  "Updates a package",
				Action: libUpdate,
			},
		},
		Category: "project",
		Action:   libInfo,
	})
}

func install(c *cli.Context) error {
	liburl := c.Args().First()
	conf, err := project.GetCfConf("")
	if err != nil {
		return err
	}

	pcache := cache.PackageCache{}
	err = pcache.Init()
	if err != nil {
		return err
	}

	var cnf project.CfConf

	if liburl == "" {
		for _, dep := range conf.Dependencies {
			pkg, err := pcache.GetPackage(dep.Package, dep.Version, dep.Identifier)
			if err != nil {
				return err
			}

			if pkg == (cache.Package{}) {
				color.Green("Package not found locally, cloning...")
				conf, _, _, err = cache.InstallLibrary(pcache, dep.Identifier, dep.Version)
				if err != nil {
					return err
				}
			} else {
				conf, err = project.GetCfConf(pkg.Path)
				if err != nil {
					return err
				}
			}
		}
	} else {
		err = pcache.CacheScan(true)
		if err != nil {
			return err
		}

		liburl, version, err := PrepUrl(liburl)
		if err != nil {
			return err
		}

		pkg, err := pcache.GetPackage("", version, strings.TrimPrefix(liburl, "https://"))
		if err != nil {
			return err
		}

		var ident, ver string
		if pkg == (cache.Package{}) {
			color.Green("Package not found locally, cloning...")
			cnf, ident, ver, err = cache.InstallLibrary(pcache, liburl, version)
			if err != nil {
				return err
			}
		} else {
			ident = pkg.Identifier
			ver = pkg.Version
			cnf, err = project.GetCfConf(pkg.Path)
			if err != nil {
				return err
			}
		}

		dep := project.CFConfDependency{
			Package:    cnf.Name,
			Version:    ver,
			Identifier: ident,
		}

		found := false
		for _, dependency := range conf.Dependencies {
			if dependency == dep {
				found = true
				break
			}
		}

		if !found {
			conf.Dependencies = append(conf.Dependencies, dep)
			err = conf.Save("cfconf.yaml", true)
			if err != nil {
				return err
			}
		}

		fmt.Println("Added package", cnf.Name, "to the project.")
	}

	fmt.Println("--------------------------------------------------")
	fmt.Println("                  Package Details                 ")
	fmt.Println("--------------------------------------------------")
	fmt.Printf("Name        : %s\n", cnf.Name)
	fmt.Printf("Description : %s\n", cnf.Description)
	fmt.Printf("Version     : %s\n", cnf.Version)
	fmt.Printf("Main File   : %s\n", cnf.Main)
	fmt.Printf("Author      : %s\n", cnf.Author)
	fmt.Printf("License     : %s\n", cnf.License)
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

func libInfo(c *cli.Context) error {
	liburl := c.Args().First()
	var conf project.CfConf
	var err error
	if liburl == "" {
		conf, err = project.GetCfConf("")
		if err != nil {
			return err
		}
	} else {
		pcache := cache.PackageCache{}
		err = pcache.Init()
		if err != nil {
			return err
		}

		err = pcache.CacheScan(true)
		if err != nil {
			return err
		}

		pkg, err := pcache.GetPackage("", "", liburl)
		if err != nil {
			return err
		}

		if pkg == (cache.Package{}) {
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
			conf, err = project.GetCfConf(tempDir)
			if err != nil {
				return err
			}
		} else {
			conf, err = project.GetCfConf(pkg.Path)
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

func libUpdate(c *cli.Context) error {
	liburl := c.Args().First()
	conf, err := project.GetCfConf("")
	if err != nil {
		return err
	}

	pcache := cache.PackageCache{}
	err = pcache.Init()
	if err != nil {
		return err
	}

	var cnf project.CfConf

	if liburl == "" {
		for _, dep := range conf.Dependencies {
			pkg, err := pcache.GetPackage(dep.Package, dep.Version, dep.Identifier)
			if err != nil {
				return err
			}

			if pkg == (cache.Package{}) {
				color.Green("Package not found locally, cloning...")
				cnf, _, _, err = cache.InstallLibrary(pcache, dep.Identifier, dep.Version)
				if err != nil {
					return err
				}
			} else {
				color.Green("Package found locally, updating...")
				cnf, _, _, err = cache.UpdateLibrary(pcache, dep.Identifier, dep.Version)
				if err != nil {
					return err
				}
			}
		}
	} else {
		err = pcache.CacheScan(true)
		if err != nil {
			return err
		}

		liburl, version, err := PrepUrl(liburl)
		if err != nil {
			return err
		}

		pkg, err := pcache.GetPackage("", version, strings.TrimPrefix(liburl, "https://"))
		if err != nil {
			return err
		}

		var ident, ver string
		if pkg == (cache.Package{}) {
			color.Green("Package not found locally, cloning...")
			cnf, ident, ver, err = cache.InstallLibrary(pcache, liburl, version)
			if err != nil {
				return err
			}
		} else {
			color.Green("Package found locally, updating...")
			cnf, ident, ver, err = cache.UpdateLibrary(pcache, liburl, version)
			if err != nil {
				return err
			}
		}

		dep := project.CFConfDependency{
			Package:    cnf.Name,
			Version:    ver,
			Identifier: ident,
		}

		conf.Dependencies = append(conf.Dependencies, dep)
		err = conf.Save("cfconf.yaml", true)
		if err != nil {
			return err
		}

		fmt.Println("Updated package", cnf.Name, "in the project.")
	}

	fmt.Println("--------------------------------------------------")
	fmt.Println("                  Package Details                 ")
	fmt.Println("--------------------------------------------------")
	fmt.Printf("Name        : %s\n", cnf.Name)
	fmt.Printf("Description : %s\n", cnf.Description)
	fmt.Printf("Version     : %s\n", cnf.Version)
	fmt.Printf("Main File   : %s\n", cnf.Main)
	fmt.Printf("Author      : %s\n", cnf.Author)
	fmt.Printf("License     : %s\n", cnf.License)
	fmt.Println("--------------------------------------------------")

	return nil
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
			if !util.PromptYN("The directory is not empty, continue?", false) {
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

	if util.PromptYN("Use default configuration?", false) {
		conf := project.CfConf{}
		conf.CreateDefault()

		err := conf.Save(path.Join(rootDir, "cfconf.yaml"), false)
		if err != nil {
			return err
		}

		fmt.Println("Created file:", path.Join(rootDir, "cfconfig.yaml"))
	} else {
		conf := project.CfConf{}

		conf.Name = util.PromptString("Project name", "NewProject")
		conf.Description = util.PromptString("Project description", "A new CaffeineC project")
		conf.Version = util.PromptString("Project version", "1.0.0")
		conf.Main = util.PromptString("Main file", "src/main.cffc")
		conf.Author = util.PromptString("Author", "Anonymous")
		conf.License = util.PromptString("License", "MIT")

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
