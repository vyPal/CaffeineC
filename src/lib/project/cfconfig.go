package project

import (
	"os"
	"path"

	"github.com/vyPal/CaffeineC/util"
	"gopkg.in/yaml.v2"
)

type CfConf struct {
	Name         string             `yaml:"name"`
	Description  string             `yaml:"description"`
	Version      string             `yaml:"version"`
	Main         string             `yaml:"main"`
	SourceDir    string             `yaml:"source"`
	Dependencies []CFConfDependency `yaml:"dependencies"`
	Author       string             `yaml:"author"`
	License      string             `yaml:"license"`
	Scripts      map[string]string  `yaml:"scripts"`
	Compiler     CFConfCompiler     `yaml:"compiler"`
}

type CFConfCompiler struct {
	Target            string `yaml:"target"`
	OptimizationLevel int    `yaml:"optimization"`
	ClangFlags        string `yaml:"clangFlags"`
	GCCFlags          string `yaml:"gccFlags"`
	LLCFlags          string `yaml:"llcFlags"`
}

type CFConfDependency struct {
	Package    string `yaml:"package"`
	Version    string `yaml:"version"`
	Identifier string `yaml:"identifier"`
}

func (c *CfConf) CreateDefault(name string) {
	if name == "." {
		name = "NewProject"
	}
	c.Name = name
	c.Description = "A new CaffeineC project"
	c.Version = "1.0.0"
	c.Main = "src/main.cffc"
	c.SourceDir = "src"
	c.Author = "Anonymous"
	c.License = "MIT"
}

func (c *CfConf) Save(filepath string, overwrite bool) error {
	if _, err := os.Stat(filepath); !os.IsNotExist(err) {
		if overwrite || util.PromptYN(filepath+" already exists. Overwrite?", false) {
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
