#!/usr/bin/env bash
# Installs the external tools pinned in tools.lock through go-qa itself (go-qa install-tools), so
# there is one install path: the checksum-verified, version-keyed tool directory that go-qa also
# resolves its tools from.
#
# Usage: scripts/install-tools.sh [--check] [--print-path] [tool...]
#   --check       install nothing; report the missing tools and exit 1 if any.
#   --print-path  after installing, print the directory of each tool, one per line (CI appends
#                 them to GITHUB_PATH so the tests find the tools).
#   tool...       limit to these tools.
#
# The directory is $GOQA_TOOLS_DIR, else the user cache directory (go-qa/tools).
set -euo pipefail

repo_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# Built, not `go run`: go run reports a failing program as exit 1 whatever code it returned.
scratch="$(mktemp -d)"
trap 'rm -rf "${scratch}"' EXIT
go build -C "${repo_dir}" -o "${scratch}/go-qa" ./cmd/go-qa
"${scratch}/go-qa" install-tools "$@"
