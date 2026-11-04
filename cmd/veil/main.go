package main

import (
	"os"
	"strings"

	"veil/internal/cli"
	"veil/internal/gui"
)

func main() {
	if len(os.Args) < 2 || strings.HasPrefix(os.Args[1], "-psn_") {
		gui.Run()
		return
	}
	cli.Run(os.Args)
}
