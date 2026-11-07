package main

import (
	"os"
	"strings"

	"veil/internal/app/cli"
	"veil/internal/app/gui"
)

func main() {
	if len(os.Args) < 2 || strings.HasPrefix(os.Args[1], "-psn_") || os.Args[1] == "--autostart" {
		gui.Run()
		return
	}
	cli.Run(os.Args)
}
