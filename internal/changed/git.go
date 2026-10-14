package changed

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

// changedFiles lists, sorted and without duplicates, the files changed against HEAD (or, with a
// base, since its merge base with HEAD) in the working tree, plus the untracked files that are not
// ignored. Paths are relative to dir.
func changedFiles(ctx context.Context, dir, base string) ([]string, error) {
	against := "HEAD"
	if base != "" {
		mergeBase, err := git(ctx, dir, "merge-base", base, "HEAD")
		if err != nil {
			return nil, fmt.Errorf("--base %s: %w", base, err)
		}
		against = strings.TrimSpace(mergeBase)
	}
	tracked, err := git(ctx, dir, "diff", "--name-only", "--no-renames", "--relative", "-z", against, "--")
	if err != nil {
		return nil, fmt.Errorf("listing the changed files: %w", err)
	}
	untracked, err := git(ctx, dir, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return nil, fmt.Errorf("listing the untracked files: %w", err)
	}
	var files []string
	for _, listing := range []string{tracked, untracked} {
		for name := range strings.SplitSeq(listing, "\x00") {
			if name != "" {
				files = append(files, filepath.ToSlash(name))
			}
		}
	}
	slices.Sort(files)
	return slices.Compact(files), nil
}

// git runs git in dir and returns its standard output; a failure carries git's own message.
func git(ctx context.Context, dir string, args ...string) (string, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}
