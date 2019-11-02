package glm

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"sync"
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

	wg := sync.WaitGroup{}

	for i, f := range funcs {
		wg.Add(1)
		go func(i int, f packageCollectorFunc) {
			defer wg.Done()

			// TODO: handle error
			packages, _ := f()
			results[i] = packages
		}(i, f)
	}

	wg.Wait()

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

	wg := sync.WaitGroup{}
	list := make([][]byte, len(m.Require))

	for i, req := range m.Require {
		wg.Add(1)
		go func(i int, path string) {
			defer wg.Done()

			cmd := exec.Command("go", "list", "-mod=readonly", fmt.Sprintf("%s/...", path))
			o, err := cmd.Output()
			if err != nil {
				list[i] = nil
				return
			}

			list[i] = o
		}(i, req.Path)
	}

	wg.Wait()

	var res []byte
	for _, r := range list {
		res = append(res, r...)
	}

	return res, nil
}
