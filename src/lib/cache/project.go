package cache

import (
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"
)

type Project struct {
	ID      string
	Path    string
	ObjDir  string
	SumFile string
}

type BuiltFile struct {
	FilePath string
	ObjPath  string
}

func (pc *PackageCache) CreateProject(p string) (Project, error) {
	err := os.MkdirAll(path.Join(pc.ObjDir, "local"), 0755)
	if err != nil {
		return Project{}, err
	}

	h := md5.New()
	io.Copy(h, strings.NewReader(p))

	return Project{
		ID:     fmt.Sprintf("%x", h.Sum(nil)),
		Path:   p,
		ObjDir: path.Join(pc.ObjDir, "local"),
	}, nil
}

func (p *Project) SaveBuiltFiles(files []BuiltFile, isCached []string) error {
	var wg sync.WaitGroup
	results := make(chan string, len(files))
	errors := make(chan error, len(files))

	pathFiles := []string{}

	for _, file := range files {
		fp := file.FilePath
		if strings.HasSuffix(fp, ".ll") {
			fp = strings.TrimSuffix(fp, ".ll") + ".cffc"
		}
		pathFiles = append(pathFiles, fp)
		if slices.Contains(isCached, file.FilePath) {
			continue
		}
		wg.Add(1)
		go func(file BuiltFile) {
			defer wg.Done()

			newPath := path.Join(p.ObjDir, filepath.Dir(file.FilePath), strings.TrimSuffix(filepath.Base(file.FilePath), ".ll")+".o")
			cmd := exec.Command("rm", "-f", path.Join(p.ObjDir, filepath.Dir(file.FilePath), strings.TrimSuffix(filepath.Base(file.FilePath), ".ll")+".o"))
			err := cmd.Run()
			if err != nil {
				errors <- err
				return
			}

			objFile, err := os.Open(file.ObjPath)
			if err != nil {
				errors <- err
				return
			}
			defer objFile.Close()

			err = os.MkdirAll(filepath.Dir(newPath), 0755)
			if err != nil {
				errors <- err
				return
			}

			newObjFile, err := os.Create(newPath)
			if err != nil {
				errors <- err
				return
			}
			defer newObjFile.Close()

			_, err = io.Copy(newObjFile, objFile)
			if err != nil {
				errors <- err
				return
			}

			results <- newPath
		}(file)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		cmd := exec.Command("sh", "-c", fmt.Sprintf("md5sum %s > %s", strings.Join(pathFiles, " "), path.Join(p.ObjDir, p.ID+".txt")))
		p.SumFile = path.Join(p.ObjDir, p.ID+".txt")
		err := cmd.Run()
		if err != nil {
			errors <- err
			return
		}
	}()

	wg.Wait()
	close(results)
	close(errors)

	if len(errors) > 0 {
		return <-errors
	}

	return nil
}

func (p *Project) SumDiff() (isCached []string, err error) {
	if p.SumFile == "" {
		p.SumFile = path.Join(p.ObjDir, p.ID+".txt")
	}
	cmd := exec.Command("md5sum", "-c", p.SumFile)
	cmd.Env = append(cmd.Env, "LANG=en")
	out, _ := cmd.Output()

	isCached = []string{}

	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "/") {
			split := strings.Split(line, ":")
			path := split[0]
			state := strings.TrimSpace(split[1])
			if state == "OK" {
				isCached = append(isCached, strings.TrimPrefix(path, "U"))
			}
		}
	}

	return isCached, nil
}

func (p *Project) GetBuiltFiles(isCached []string) (builtFiles []BuiltFile, err error) {
	for _, cachedFile := range isCached {
		file := strings.TrimSuffix(cachedFile, ".cffc") + ".o"

		objPath := path.Join(p.ObjDir, file)

		builtFiles = append(builtFiles, BuiltFile{FilePath: cachedFile, ObjPath: objPath})
	}

	return builtFiles, nil
}
