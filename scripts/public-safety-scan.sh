#!/usr/bin/env bash
# Public-safety scan: fails when repository content matches any term in a denylist that is kept
# OUTSIDE this repository (so the list itself is never published).
#
# Usage:
#   GQACI_DENYLIST=/path/to/denylist scripts/public-safety-scan.sh [mode]
#   (or once per clone: git config gqaci.denylist /path/to/denylist)
# Modes:
#   --tree           every tracked file as git stores it (symlinks as their target string, binary
#                    files included), contents and paths (default)
#   --staged         staged changes (pre-commit)
#   --message FILE   a commit message file (commit-msg)
#   --history        every ref: commit diffs, messages, author and committer identities, annotated
#                    tag messages, notes and ref names (pre-push, CI, pre-publication audit)
#   --stdin          text on standard input (issue and PR text before posting, CI on issue events)
#
# Denylist format: one extended regular expression per line, matched case-insensitively. Carriage
# returns and leading or trailing whitespace are stripped; blank lines and lines whose first
# non-blank character is '#' are ignored. docs/denylist.example shows the format with
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
# Resolve the file itself, so a symlink to a file inside the repository is refused too.
case "$(realpath -- "${denylist}")" in
    "$(realpath -- "${repo_dir}")/"*)
        echo "public-safety-scan: the denylist must live outside the repository" >&2
        exit 2
        ;;
esac

# One predicate for the whole script: a denylist line is a pattern unless, after stripping carriage
# returns and surrounding whitespace, it is empty or a comment.
patterns=()
numbers=()
number=0
while IFS= read -r raw || [[ -n "${raw}" ]]; do
    number=$((number + 1))
    line="${raw//$'\r'/}"
    line="${line#"${line%%[![:space:]]*}"}"
    line="${line%"${line##*[![:space:]]}"}"
    [[ -z "${line}" || "${line}" == \#* ]] && continue
    status=0
    grep -E -- "${line}" /dev/null || status=$?
    if (( status > 1 )); then
        echo "public-safety-scan: denylist line ${number} is not a valid extended regular expression" >&2
        exit 2
    fi
    patterns+=("${line}")
    numbers+=("${number}")
done < "${denylist}"
if (( ${#patterns[@]} == 0 )); then
    echo "public-safety-scan: the denylist has no patterns; an empty list would pass everything" >&2
    exit 2
fi

mode="${1:---tree}"
work="$(mktemp -d)"
trap 'rm -rf "${work}"' EXIT
text="${work}/text"
locations="${work}/locations"
paths="${work}/paths"

case "${mode}" in
    --tree)
        # text holds blob contents only; locations and paths are kept in parallel files so the
        # report can name a file by index when the file name itself matches a pattern. Contents
        # come from git, never from the working file, so a symlink is scanned as its target
        # string and nothing outside the repository is read.
        : > "${text}"
        : > "${locations}"
        : > "${paths}"
        index=0
        while IFS= read -r -d '' entry; do
            read -r file_mode blob_id _ <<< "${entry%%$'\t'*}"
            file="${entry#*$'\t'}"
            index=$((index + 1))
            printf '%s\n' "${file}" >> "${paths}"
            [[ "${file_mode}" == 160000 ]] && continue # a submodule pin has no blob here
            git -C "${repo_dir}" cat-file blob "${blob_id}" \
                | GQ_INDEX="${index}" GQ_FILE="${file}" GQ_OUT="${locations}" \
                    awk '{ print $0; print ENVIRON["GQ_INDEX"] "\t" FNR "\t" ENVIRON["GQ_FILE"] >> ENVIRON["GQ_OUT"] }' \
                    >> "${text}"
        done < <(git -C "${repo_dir}" ls-files -s -z)
        ;;
    --staged)
        git -C "${repo_dir}" diff --cached --text --unified=0 --no-color > "${text}"
        git -C "${repo_dir}" diff --cached --name-only >> "${text}"
        ;;
    --message)
        cp "${2:?--message needs a file}" "${text}"
        ;;
    --history)
        git -C "${repo_dir}" log --all -m --no-color -p --text \
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

# True when any denylist pattern matches the argument; used so a path is never printed when it
# would republish a term, whichever denylist line the hit came from.
matches_any_pattern() {
    local candidate="$1" pattern
    for pattern in "${patterns[@]}"; do
        grep -qiE -- "${pattern}" <<< "${candidate}" && return 0
    done
    return 1
}

found=0
for i in "${!patterns[@]}"; do
    pattern="${patterns[i]}"
    number="${numbers[i]}"
    status=0
    matches="$(grep -naiE -- "${pattern}" "${text}")" || status=$?
    if (( status > 1 )); then
        echo "public-safety-scan: grep failed on denylist line ${number}" >&2
        exit 2
    fi
    if (( status == 0 )); then
        found=1
        while IFS= read -r hit; do
            line_number="${hit%%:*}"
            if [[ "${mode}" == "--tree" ]]; then
                location="$(awk -v n="${line_number}" 'NR == n { print; exit }' "${locations}")"
                if [[ -z "${location}" ]]; then
                    echo "public-safety-scan: no file location for scanned text line ${line_number}; refusing to guess" >&2
                    exit 2
                fi
                IFS=$'\t' read -r file_index file_line file_name <<< "${location}"
                if matches_any_pattern "${file_name}"; then
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
        path_matches="$(grep -naiE -- "${pattern}" "${paths}")" || status=$?
        if (( status == 0 )); then
            found=1
            while IFS= read -r hit; do
                echo "public-safety: denylist line ${number} matched at tracked path #${hit%%:*}"
            done <<< "${path_matches}"
        fi
    fi
done

if (( found )); then
    echo "public-safety-scan: FAILED (${mode}); remove the matched content before it is committed or published" >&2
    exit 1
fi
echo "public-safety-scan: clean (${mode})"
