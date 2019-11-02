package main

import (
	"fmt"
	"os"

	"github.com/110y/glm/pkg/glm"
)

func main() {
	output, err := glm.GetImportablePackages()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to exec glm: %s", err.Error())
		os.Exit(1)
	}

	fmt.Fprint(os.Stdout, string(output))
}
