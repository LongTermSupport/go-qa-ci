#!/usr/bin/env bash
# Installs the pinned golangci-lint release into ${RUNNER_TEMP}/golangci-lint and puts it on PATH,
# after verifying the archive against the checksum file published with the release.
# Requires GOLANGCI_LINT_VERSION (for example 2.14.0), GH_TOKEN, RUNNER_TEMP and GITHUB_PATH.
set -euo pipefail

: "${GOLANGCI_LINT_VERSION:?set GOLANGCI_LINT_VERSION}"
: "${RUNNER_TEMP:?set RUNNER_TEMP}"
: "${GITHUB_PATH:?set GITHUB_PATH}"

archive="golangci-lint-${GOLANGCI_LINT_VERSION}-linux-amd64.tar.gz"
checksums="golangci-lint-${GOLANGCI_LINT_VERSION}-checksums.txt"
dest="${RUNNER_TEMP}/golangci-lint"

mkdir -p "${dest}"
cd "${dest}"
gh release download "v${GOLANGCI_LINT_VERSION}" -R golangci/golangci-lint -p "${archive}" -p "${checksums}"
sha256sum -c --ignore-missing "${checksums}"
tar xzf "${archive}" --strip-components=1 "golangci-lint-${GOLANGCI_LINT_VERSION}-linux-amd64/golangci-lint"
echo "${dest}" >> "${GITHUB_PATH}"
