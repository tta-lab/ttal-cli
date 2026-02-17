package main

import (
	"os"

	"codeberg.org/clawteam/ttal-cli/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
