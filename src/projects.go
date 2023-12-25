package main

import (
	"bufio"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/urfave/cli/v2"
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
	})
}

type CfConf struct {
	Name         string   `yaml:"name"`
	Description  string   `yaml:"description"`
	Version      string   `yaml:"version"`
	Main         string   `yaml:"main"`
	Dependencies []string `yaml:"dependencies"`
	Author       string   `yaml:"author"`
	License      string   `yaml:"license"`
}

func (c *CfConf) CreateDefault() {
	c.Name = "NewProject"
	c.Description = "A new CaffeineC project"
	c.Version = "1.0.0"
	c.Main = "src/main.cffc"
	c.Author = "Anonymous"
	c.License = "MIT"
}

func (c *CfConf) Save(filepath string) error {
	if _, err := os.Stat(filepath); !os.IsNotExist(err) {
		if promptYN(filepath+" already exists. Overwrite?", false) {
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

		fmt.Println("Created file:", path.Join(rootDir, "src", "main.cffc"))
	}

	if promptYN("Use default configuration?", false) {
		conf := CfConf{}
		conf.CreateDefault()

		err := conf.Save(path.Join(rootDir, "cfconf.yaml"))
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

		err := conf.Save(path.Join(rootDir, "cfconf.yaml"))
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
