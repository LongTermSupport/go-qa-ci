// Package goqaci carries tools.lock inside the go-qa binary, so a consumer's go-qa knows the
// tool versions and checksums it was built with. The file in the repository is the only copy.
package goqaci

import _ "embed" // the blank import go:embed needs

// ToolsLock is the text of tools.lock.
//
//go:embed tools.lock
var ToolsLock string
