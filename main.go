package main

import (
	"os"

	"github.com/jconder44/dockflux/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
