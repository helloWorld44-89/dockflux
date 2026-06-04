package main

import (
	"os"

	"github.com/darkmode_dev/dockflux/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
