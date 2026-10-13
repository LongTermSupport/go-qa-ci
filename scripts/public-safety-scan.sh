#!/usr/bin/env bash
# Public-safety scan: fails when repository content matches any term in a denylist that is kept
# OUTSIDE this repository (so the list itself is never published).
#
# Usage:
#   GQACI_DENYLIST=/path/to/denylist scripts/public-safety-scan.sh [mode]
#   (or once per clone: git config gqaci.denylist /path/to/denylist)
# Modes:
#   --tree           tracked files in the working tree, contents and paths (default)
#   --staged         staged changes (pre-commit)
#   --message FILE   a commit message file (commit-msg)
#   --history        every ref: commit diffs, messages, author and committer identities, annotated
#                    tag messages, notes and ref names (pre-push, CI, pre-publication audit)
#   --stdin          text on standard input (issue and PR text before posting, CI on issue events)
#
# Denylist format: one extended regular expression per line, matched case-insensitively; blank
# lines and lines starting with '#' are ignored. docs/denylist.example shows the format with
# placeholder terms. Output names the location and the denylist line number, never the matched
# term, so logs do not republish what the list protects.
set -euo pipefail

denylist="${GQACI_DENYLIST:-}"
if [[ -z "${denylist}" ]] && configured="$(git config --get gqaci.denylist)"; then
    denylist="${configured}"
fi
if [[ -z "${denylist}" || ! -r "${denylist}" ]]; then
    echo "public-safety-scan: set GQACI_DENYLIST (or git config gqaci.denylist) to a readable denylist file outside the repo" >&2
    exit 2
fi
repo_dir="$(git rev-parse --show-toplevel)"
case "$(cd "$(dirname "${denylist}")" && pwd -P)/" in
    "${repo_dir}/"*)
        echo "public-safety-scan: the denylist must live outside the repository" >&2
        exit 2
        ;;
esac
if ! grep -qvE '^[[:space:]]*(#|$)' "${denylist}"; then
    echo "public-safety-scan: the denylist has no patterns; an empty list would pass everything" >&2
    exit 2
fi

mode="${1:---tree}"
work="$(mktemp -d)"
trap 'rm -rf "${work}"' EXIT
text="${work}/text"

case "${mode}" in
    --tree)
        # text holds file contents only; locations and paths are kept in parallel files so the
        # report can name a file by index when the file name itself matches a pattern.
        locations="${work}/locations"
        paths="${work}/paths"
        : > "${text}" > "${locations}" > "${paths}"
        index=0
        while IFS= read -r -d '' file; do
            index=$((index + 1))
            printf '%s\n' "${file}" >> "${paths}"
            if [[ -f "${repo_dir}/${file}" ]] && grep -Iq . "${repo_dir}/${file}" 2>/dev/null; then
                awk -v i="${index}" -v f="${file}" -v out="${locations}" \
                    '{ print $0; print i "\t" FNR "\t" f > out }' "${repo_dir}/${file}" >> "${text}"
            fi
        done < <(git -C "${repo_dir}" ls-files -z)
        ;;
    --staged)
        git -C "${repo_dir}" diff --cached --unified=0 --no-color > "${text}"
        git -C "${repo_dir}" diff --cached --name-only >> "${text}"
        ;;
    --message)
        cp "${2:?--message needs a file}" "${text}"
        ;;
    --history)
        git -C "${repo_dir}" log --all --no-color -p \
            --format='commit %H%nauthor %an <%ae>%ncommitter %cn <%ce>%n%B' > "${text}"
        git -C "${repo_dir}" for-each-ref \
            --format='ref %(refname)%0atagger %(taggername) %(taggeremail)%0a%(contents)' >> "${text}"
        ;;
    --stdin)
        cat > "${text}"
        ;;
    *)
        echo "public-safety-scan: unknown mode ${mode}" >&2
        exit 2
        ;;
esac

found=0
number=0
while IFS= read -r pattern || [[ -n "${pattern}" ]]; do
    number=$((number + 1))
    [[ -z "${pattern// /}" || "${pattern}" == \#* ]] && continue
    status=0
    matches="$(grep -inE -- "${pattern}" "${text}")" || status=$?
    if (( status > 1 )); then
        echo "public-safety-scan: denylist line ${number} is not a valid extended regular expression" >&2
        exit 2
    fi
    if (( status == 0 )); then
        found=1
        while IFS= read -r hit; do
            line_number="${hit%%:*}"
            if [[ "${mode}" == "--tree" ]]; then
                IFS=$'\t' read -r file_index file_line file_name < <(awk -v n="${line_number}" 'NR == n' "${locations}")
                if grep -qiE -- "${pattern}" <<< "${file_name}"; then
                    where="tracked path #${file_index}:${file_line}"
                else
                    where="${file_name}:${file_line}"
                fi
            else
                where="${mode#--} text line ${line_number}"
            fi
            echo "public-safety: denylist line ${number} matched at ${where}"
        done <<< "${matches}"
    fi
    if [[ "${mode}" == "--tree" ]]; then
        status=0
        path_matches="$(grep -inE -- "${pattern}" "${paths}")" || status=$?
        if (( status == 0 )); then
            found=1
            while IFS= read -r hit; do
                echo "public-safety: denylist line ${number} matched at tracked path #${hit%%:*}"
            done <<< "${path_matches}"
        fi
    fi
done < "${denylist}"

if (( found )); then
    echo "public-safety-scan: FAILED (${mode}); remove the matched content before it is committed or published" >&2
    exit 1
fi
echo "public-safety-scan: clean (${mode})"
