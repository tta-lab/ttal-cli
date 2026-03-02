package main

import (
	"os"

	"github.com/tta-lab/ttal-cli/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
