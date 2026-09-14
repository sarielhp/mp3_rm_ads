package main

import (
	"os"

	"pod/pkg/cli"
)

func main() {
	os.Exit(cli.Execute(os.Args[1:]))
}
