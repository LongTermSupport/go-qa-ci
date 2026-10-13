// Package configs holds the configuration files bundled with go-qa.
package configs

import _ "embed"

//go:embed golangci.yml
var golangci []byte

// Golangci returns the strict golangci-lint v2 baseline used when a project has no configuration
// of its own.
func Golangci() []byte {
	return append([]byte(nil), golangci...)
}
