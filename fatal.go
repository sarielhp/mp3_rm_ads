package main

import (
	"fmt"
	"os"
)

func fatalError(msg string, args ...interface{}) {
	if len(args) > 0 {
		fmt.Fprintf(os.Stderr, msg, args...)
	} else {
		fmt.Fprint(os.Stderr, msg)
	}
	os.Exit(1)
}
