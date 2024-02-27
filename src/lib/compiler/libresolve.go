package compiler

import (
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/vyPal/CaffeineC/lib/cache"
	"github.com/vyPal/CaffeineC/lib/project"
)

func ResolveImportPath(path string, pcache cache.PackageCache) (cffcpath string, importpath string, err error) {
	prefixes := []string{"./", "/", "../"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(path, prefix) {
			return path, path, nil
		}
	}

	found, pkg, fp, objdir, err := pcache.ResolvePackage(path)
	if err != nil {
		return "", "", err
	}
	if found {
		conf, err := project.GetCfConf(pkg.Path)
		if err != nil {
			return "", "", err
		}
		if conf.SourceDir == "" {
			color.Yellow("Package %s doesn't have a configured source directory. Using src/", pkg.Name)
			conf.SourceDir = "src"
		}
		if !strings.HasSuffix(fp, ".cffc") {
			fp += ".cffc"
		}
		if objdir == "" {
			objdir = filepath.Join(pkg.Path, conf.SourceDir, fp)
		}
		cffcpath = filepath.Join(pkg.Path, conf.SourceDir, fp)
		return cffcpath, objdir, nil
	} else {
		var builder strings.Builder
		builder.WriteString("./")
		builder.WriteString(path)
		return builder.String(), builder.String(), nil
	}
}
