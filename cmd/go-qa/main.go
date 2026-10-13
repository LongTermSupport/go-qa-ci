// Command go-qa is the Defence Before Fix toolchain entry point for Go modules.
package main

import (
	"os"

	"github.com/LongTermSupport/go-qa-ci/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
