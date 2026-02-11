package main

import (
	"os"

	"github.com/guion-opensource/ttal-cli/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
