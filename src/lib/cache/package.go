package cache

import (
	"encoding/gob"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/vyPal/CaffeineC/lib/project"
	"github.com/vyPal/CaffeineC/util"
)

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
			conf, err := project.GetCfConf(path)
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
