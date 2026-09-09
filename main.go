package main

import (
	"os"

	"github.com/sariel/abs/pkg/cli"
)

func main() {
	os.Exit(cli.Execute(os.Args[1:]))
}
