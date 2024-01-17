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
