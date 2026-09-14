package main

import (
	_ "embed"
	"os"
	"strings"

	"pod/pkg/cli"
)

//go:embed VERSION
var version string

func main() {
	cli.SetEmbeddedVersion(strings.TrimSpace(version))
	os.Exit(cli.Execute(os.Args[1:]))
}
