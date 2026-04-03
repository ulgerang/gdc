package main

import (
	"os"

	"github.com/gdc-tools/gdc/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
