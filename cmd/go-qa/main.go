// Command go-qa is the Defence Before Fix toolchain entry point for Go modules.
//
// Run with GQACI_VETTOOL=1 in the environment the binary is instead a go vet tool carrying the
// bundled analyzers: the analyzers detector starts go vet with the binary as its -vettool.
package main

import (
	"os"

	"github.com/LongTermSupport/go-qa-ci/analyzers/sensitiveformat"
	"github.com/LongTermSupport/go-qa-ci/driver"
	"github.com/LongTermSupport/go-qa-ci/internal/cli"
	"github.com/LongTermSupport/go-qa-ci/internal/vet"
)

func main() {
	if vet.IsVettool() {
		driver.Main(sensitiveformat.Analyzer)
	}
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
