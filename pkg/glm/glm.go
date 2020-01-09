package glm

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"

	"golang.org/x/sync/errgroup"
)

type mod struct {
	Require []*require
}

type require struct {
	Path string
}

type packageCollectorFunc func() ([]byte, error)

var usableStdPkgRegex = regexp.MustCompile("(^vendor|^.*?internal)")

func GetImportablePackages() ([]byte, error) {
	funcs := []packageCollectorFunc{
		listStandardPackages,
		listProjectPackages,
		listModulePackages,
	}

	results := make([][]byte, len(funcs))

	var eg errgroup.Group

	for i, f := range funcs {
		i, f := i, f
		eg.Go(func() error {
			packages, err := f()
			if err != nil {
				return err
			}

			results[i] = packages
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	var output []byte
	for _, result := range results {
		output = append(output, result...)
	}

	return output, nil
}

func listStandardPackages() ([]byte, error) {
	o, err := exec.Command("go", "list", "std").Output()
	if err != nil {
		return nil, err
	}

	return o, nil
}

func listProjectPackages() ([]byte, error) {
	o, err := exec.Command("go", "list", "./...").Output()
	if err != nil {
		return nil, err
	}

	return o, nil
}

func listModulePackages() ([]byte, error) {
	modsJSON, err := exec.Command("go", "mod", "edit", "-json").Output()
	if err != nil {
		return nil, err
	}

	var m mod
	if err := json.Unmarshal(modsJSON, &m); err != nil {
		return nil, err
	}

	var eg errgroup.Group
	list := make([][]byte, len(m.Require))

	for i, req := range m.Require {
		i := i
		req := req
		eg.Go(func() error {
			modfile := os.Getenv("MODFILE")
			if modfile == "" {
				modfile = "go.mod"
			}

			cmd := exec.Command("go", "list", fmt.Sprintf("-modfile=%s", modfile), fmt.Sprintf("%s/...", req.Path))
			o, err := cmd.Output()
			if err != nil {
				list[i] = nil
				return fmt.Errorf("failed to execute `go list`: %s", err.Error())
			}

			list[i] = o
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	var res []byte
	for _, r := range list {
		res = append(res, r...)
	}

	return res, nil
}
