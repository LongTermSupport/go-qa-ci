#!/usr/bin/env bash
# Installs the external tools at the versions pinned in tools.lock.
#
# Usage: scripts/install-tools.sh [--check] [tool...]
#   --check  only report missing or mismatched tools; exit 1 if any.
#   tool...  limit to these tools (for example `golangci-lint` in a CI job that needs only that).
#
# Tools land in ${GOBIN}, else $(go env GOPATH)/bin. Release archives are linux/amd64 only; the
# [go] tools build anywhere Go does. GOTOOLCHAIN is left as the environment sets it.
set -euo pipefail

repo_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
lock="${GQACI_TOOLS_LOCK:-${repo_dir}/tools.lock}"
check_only=0
if [[ "${1:-}" == "--check" ]]; then
    check_only=1
    shift
fi
selected=("$@")

# Prints "section name value" for every line of the lock.
pins() {
    awk -F'"' '
        /^\[/ { gsub(/[\[\]]/, ""); section = $0; next }
        /^[a-z]/ { name = $1; sub(/ *=.*/, "", name); print section, name, $2 }
    ' "${lock}"
}

# The value pinned for a name in a section, or empty.
pinned() {
    pins | awk -v s="$1" -v n="$2" '$1 == s && $2 == n { print $3 }'
}

wanted() {
    local name="$1" choice
    if [[ ${#selected[@]} -eq 0 ]]; then
        return 0
    fi
    for choice in "${selected[@]}"; do
        [[ "${choice}" == "${name}" ]] && return 0
    done
    return 1
}

for choice in "${selected[@]}"; do
    if [[ -z "$(pinned go "${choice}")$(pinned release "${choice}")" ]]; then
        echo "install-tools: ${choice} is not pinned in tools.lock" >&2
        exit 2
    fi
done

# The version an installed tool reports; empty when it is not on PATH.
installed_version() {
    local section="$1" name="$2" bin
    if ! bin="$(command -v "${name}")"; then
        return 0
    fi
    case "${section}:${name}" in
        go:*) go version -m "${bin}" | awk '$1 == "mod" { print $3; exit }' ;;
        release:golangci-lint) "${bin}" --version | awk '{ for (i = 1; i < NF; i++) if ($i == "version") { print $(i + 1); exit } }' ;;
        release:ast-grep) "${bin}" --version | awk '{ print $2; exit }' ;;
        *) echo "install-tools: no version probe for ${name}" >&2; exit 2 ;;
    esac
}

bin_dir() {
    echo "${GOBIN:-$(go env GOPATH)/bin}"
}

require_linux_amd64() {
    if [[ "$(uname -s)-$(uname -m)" != "Linux-x86_64" ]]; then
        echo "install-tools: release archives are pinned for linux/amd64 only, not $(uname -s)-$(uname -m)" >&2
        exit 2
    fi
}

fetch() {
    curl --proto '=https' --tlsv1.2 -sSfL -o "$2" "$1"
}

install_golangci_lint() {
    local version="$1" work archive checksums base
    require_linux_amd64
    work="$(mktemp -d)"
    trap 'rm -rf "${work}"' RETURN
    archive="golangci-lint-${version}-linux-amd64.tar.gz"
    checksums="golangci-lint-${version}-checksums.txt"
    base="https://github.com/golangci/golangci-lint/releases/download/v${version}"
    fetch "${base}/${archive}" "${work}/${archive}"
    fetch "${base}/${checksums}" "${work}/${checksums}"
    (cd "${work}" && sha256sum -c --ignore-missing "${checksums}")
    tar -xzf "${work}/${archive}" -C "${work}" --strip-components=1 "golangci-lint-${version}-linux-amd64/golangci-lint"
    install -D -m 0755 "${work}/golangci-lint" "$(bin_dir)/golangci-lint"
}

install_ast_grep() {
    local version="$1" work sum
    require_linux_amd64
    sum="$(pinned checksums ast-grep)"
    if [[ -z "${sum}" ]]; then
        echo "install-tools: ast-grep publishes no checksum file; tools.lock [checksums] must pin its sha256" >&2
        exit 2
    fi
    work="$(mktemp -d)"
    trap 'rm -rf "${work}"' RETURN
    fetch "https://github.com/ast-grep/ast-grep/releases/download/${version}/app-x86_64-unknown-linux-gnu.zip" "${work}/app.zip"
    echo "${sum}  ${work}/app.zip" | sha256sum -c -
    unzip -q "${work}/app.zip" ast-grep -d "${work}"
    install -D -m 0755 "${work}/ast-grep" "$(bin_dir)/ast-grep"
}

missing=0
while read -r section name value; do
    [[ "${section}" == "go" || "${section}" == "release" ]] || continue
    wanted "${name}" || continue
    if [[ "${section}" == "go" ]]; then
        version="${value##*@}"
    else
        version="${value}"
    fi
    current="$(installed_version "${section}" "${name}")"
    if [[ "${current}" == "${version}" ]]; then
        echo "ok        ${name} ${version}"
        continue
    fi
    if (( check_only )); then
        echo "MISSING   ${name} ${version} (installed: ${current:-none})"
        missing=1
        continue
    fi
    echo "installing ${name} ${version}" >&2
    case "${section}:${name}" in
        go:*) GOBIN="$(bin_dir)" go install "${value}" ;;
        release:golangci-lint) install_golangci_lint "${version}" ;;
        release:ast-grep) install_ast_grep "${version}" ;;
        *) echo "install-tools: no installer for ${name}" >&2; exit 2 ;;
    esac
    echo "installed ${name} ${version}"
done < <(pins)

exit "${missing}"
