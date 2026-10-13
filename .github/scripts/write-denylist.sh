#!/usr/bin/env bash
# Writes the GQACI_DENYLIST secret (passed as $DENYLIST) to a private file outside the checkout and
# exports its path as GQACI_DENYLIST for later steps. A missing secret fails the job, except on pull
# requests from forks, which receive no secrets: those are scanned again when merged.
set -euo pipefail

if [[ -z "${DENYLIST:-}" ]]; then
    if [[ "${FORK_PR:-false}" == "true" ]]; then
        echo "::warning::GQACI_DENYLIST is not available to pull requests from forks; the denylist scan runs after merge"
        exit 0
    fi
    echo "::error::repository secret GQACI_DENYLIST is not set; the public-safety denylist scan cannot run"
    exit 1
fi
umask 077
path="${RUNNER_TEMP:?}/public-safety-denylist"
printf '%s\n' "${DENYLIST}" > "${path}"
echo "GQACI_DENYLIST=${path}" >> "${GITHUB_ENV:?}"
