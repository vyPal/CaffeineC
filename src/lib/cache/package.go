package cache

import (
	"encoding/gob"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/vyPal/CaffeineC/lib/project"
)

type PackageCache struct {
	BaseDir string
	RootDir string
	ObjDir  string
	PkgList []Package
}

type Package struct {
	Name       string
	Version    string
	Identifier string
	Path       string
	ObjDir     string
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

	objDir := path.Join(libDir, "obj")
	err = os.Mkdir(objDir, 0700)
	if err != nil && !os.IsExist(err) {
		return err
	}

	p.RootDir = libDir
	p.BaseDir = cacheDir
	p.ObjDir = objDir
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
			split := strings.Split(identifier, "/")
			branch := split[len(split)-1]
			identifier = strings.TrimSuffix(identifier, "/"+branch)

			objDir := filepath.Join(p.ObjDir, identifier)

			if _, err := os.Stat(objDir); !os.IsNotExist(err) {
				os.RemoveAll(objDir)
			}

			os.MkdirAll(objDir, 0755)

			cmd := exec.Command("CaffeineC", "build", "--obj", path)
			cmd.Dir = objDir
			err = cmd.Run()
			if err != nil {
				return err
			}

			pkg := Package{
				Name:       conf.Name,
				Version:    branch,
				Identifier: identifier,
				Path:       path,
				ObjDir:     objDir,
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
	cacheFile, err := os.Open(path.Join(p.RootDir, "cache.bin"))
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
	cacheFile, err := os.Create(path.Join(p.RootDir, "cache.bin"))
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

func (p *PackageCache) FindPackage(name, version, identifier string) (Package, bool, error) {
	for _, pkg := range p.PkgList {
		if (pkg.Name == name || name == "") && (pkg.Identifier == identifier || pkg.Identifier == "github.com/"+identifier) {
			if version == "" || version == "*" || version == pkg.Version {
				return pkg, true, nil
			}
			continue
		}
	}

	return Package{}, false, nil
}

func (p *PackageCache) ResolvePackage(ident string) (found bool, pkg Package, fp string, objDir string, err error) {
	split := strings.Split(ident, "/")
	for i := len(split); i > 0; i-- {
		joined := strings.Join(split[:i], "/")
		pkg, found, err = p.FindPackage("", "*", joined)
		if err != nil {
			return false, Package{}, "", "", err
		}
		if found {
			objDir = pkg.ObjDir
			fp = strings.Join(split[i:], "/")
			break
		}
	}
	return found, pkg, fp, objDir, nil
}

func UpdateLibrary(pcache PackageCache, liburl string, version string) (conf project.CfConf, ident, ver string, e error) {
	// Get the directory in the cache's BaseDir
	updateDir := filepath.Join(pcache.BaseDir, strings.TrimPrefix(liburl, "https://"), version)

	// Open the existing repository
	repo, err := git.PlainOpen(updateDir)
	if err != nil {
		return project.CfConf{}, "", "", err
	}

	// Get the working directory for the repository
	w, err := repo.Worktree()
	if err != nil {
		return project.CfConf{}, "", "", err
	}

	// Pull the latest changes from the origin
	err = w.Pull(&git.PullOptions{
		RemoteName:    "origin",
		ReferenceName: plumbing.NewBranchReferenceName(version),
	})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return project.CfConf{}, "", "", err
	}

	objDir := filepath.Join(pcache.ObjDir, strings.TrimPrefix(liburl, "https://"), version)

	// If the obj directory exists, clear it
	if _, err := os.Stat(objDir); !os.IsNotExist(err) {
		os.RemoveAll(objDir)
	}

	// Create the obj directory
	os.MkdirAll(objDir, 0700)

	// Run the CaffeineC build command
	cmd := exec.Command("CaffeineC", "build", "--obj", "-c", updateDir)
	cmd.Dir = objDir
	err = cmd.Run()
	if err != nil {
		return project.CfConf{}, "", "", err
	}

	// Get the configuration file from the updated repository
	conf, err = project.GetCfConf(updateDir)
	if err != nil {
		return project.CfConf{}, "", "", err
	}

	return conf, strings.TrimPrefix(liburl, "https://"), version, nil
}

func InstallLibrary(pcache PackageCache, liburl string, version string) (conf project.CfConf, ident, ver string, e error) {
	// Create a directory in the cache's BaseDir
	installDir := filepath.Join(pcache.BaseDir, strings.TrimPrefix(liburl, "https://"), version)
	err := os.MkdirAll(installDir, 0700)
	if err != nil {
		return project.CfConf{}, "", "", err
	}

	// Clone the repository to the install directory
	_, err = git.PlainClone(installDir, false, &git.CloneOptions{
		URL:           liburl,
		SingleBranch:  true,
		Depth:         1,
		ReferenceName: plumbing.NewBranchReferenceName(version),
	})
	if err != nil {
		return project.CfConf{}, "", "", err
	}

	objDir := filepath.Join(pcache.ObjDir, strings.TrimPrefix(liburl, "https://"), version)

	// If the obj directory exists, clear it
	if _, err := os.Stat(objDir); !os.IsNotExist(err) {
		os.RemoveAll(objDir)
	}

	// Create the obj directory
	os.MkdirAll(objDir, 0700)

	// Run the CaffeineC build command
	cmd := exec.Command("CaffeineC", "build", "--obj", "-c", installDir)
	cmd.Dir = objDir
	err = cmd.Run()
	if err != nil {
		return project.CfConf{}, "", "", err
	}

	pkg := Package{
		Name:       conf.Name,
		Version:    version,
		Identifier: strings.TrimPrefix(liburl, "https://"),
		Path:       installDir,
		ObjDir:     objDir,
	}
	pcache.PkgList = append(pcache.PkgList, pkg)
	err = pcache.CacheSave()
	if err != nil {
		return project.CfConf{}, "", "", err
	}

	// Get the configuration file from the cloned repository
	conf, err = project.GetCfConf(installDir)
	if err != nil {
		return project.CfConf{}, "", "", err
	}

	return conf, strings.TrimPrefix(liburl, "https://"), version, nil
}
