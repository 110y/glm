package glm

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"

	"golang.org/x/sync/errgroup"
)

var grepArgs = []string{
	"--invert-match",
	"--extended-regexp",
	".*?internal|^vendor",
}

type mod struct {
	Require []*require
}

type require struct {
	Path string
}

type packageCollectorFunc func() ([]byte, error)

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
	cmd := exec.Command("go", "list", "std")
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stdout pipe of `go list std`: %w", err)
	}
	defer pipe.Close()

	grep := exec.Command("grep", grepArgs...)
	grep.Stdin = pipe

	if err = cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start `go list std`: %w", err)
	}

	o, err := grep.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to execut `go list std`: %w", err)
	}

	return o, nil
}

func listProjectPackages() ([]byte, error) {
	o, err := exec.Command("go", "list", "./...").Output()
	if err != nil {
		return nil, fmt.Errorf("failed to execute `go list ./...`: %w", err)
	}

	return o, nil
}

func listModulePackages() ([]byte, error) {
	modfile := os.Getenv("GLM_MOD_PATH")
	if modfile == "" {
		modfile = "./go.mod"
	}

	modsJSON, err := exec.Command("go", "mod", "edit", "-json").Output()
	if err != nil {
		return nil, err
	}

	var m mod
	if err := json.Unmarshal(modsJSON, &m); err != nil {
		return nil, err
	}

	list := make([][]byte, len(m.Require))

	isWorkspaceMode := false
	_, err = os.Stat("./go.work")
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("failed to check go.work existence: %w", err)
	}

	if err == nil {
		isWorkspaceMode = true
	}

	for i, req := range m.Require {
		cmd := createGoListForExternalModsCommand(req.Path, modfile, isWorkspaceMode)

		pipe, err := cmd.StdoutPipe()
		if err != nil {
			return nil, fmt.Errorf("failed to get stdout pipe of `go list`: %w", err)
		}
		defer pipe.Close()

		grep := exec.Command("grep", grepArgs...)
		grep.Stdin = pipe

		if err = cmd.Start(); err != nil {
			return nil, fmt.Errorf("failed to start `go list` for third party mods: %w", err)
		}

		o, err := grep.Output()
		if err != nil {
			list[i] = nil
			return nil, fmt.Errorf("failed to execute `go list` mod: %s, err:%s, %s", req.Path, err.Error(), string(o))
		}

		list[i] = o
	}

	var res []byte
	for _, r := range list {
		res = append(res, r...)
	}

	return res, nil
}

func createGoListForExternalModsCommand(mod, modfile string, isWorkspaceMode bool) *exec.Cmd {
	args := []string{
		"list",
	}

	if !isWorkspaceMode {
		args = append(args, "-mod=mod")
	}

	args = append(args, fmt.Sprintf("-modfile=%s", modfile), fmt.Sprintf("%s/...", mod))

	cmd := exec.Command("go", args...)
	cmd.Env = append(
		os.Environ(),
		"GOPROXY=direct",
		"GOSUMDB=off",
	)

	return cmd
}
