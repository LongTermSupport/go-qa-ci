// Package worktree detects whether a formatter rewrote Go files, by comparing content hashes taken
// before and after, so CI mode needs neither a clean tree to start from nor git.
package worktree

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"slices"
	"strings"
)

// Snapshot maps every Go source file under dir (slash-separated, relative) to its content hash.
// The .git directory is not walked, and nothing outside dir is read.
func Snapshot(dir string) (map[string]string, error) {
	handle, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("snapshot of %s: %w", dir, err)
	}
	hashes := map[string]string{}
	fsys := handle.FS()
	walkErr := fs.WalkDir(fsys, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return fs.SkipDir
			}
			return nil
		}
		if !entry.Type().IsRegular() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			return fmt.Errorf("reading %s: %w", path, err)
		}
		sum := sha256.Sum256(data)
		hashes[path] = hex.EncodeToString(sum[:])
		return nil
	})
	closeErr := handle.Close()
	if walkErr != nil {
		return nil, fmt.Errorf("snapshot of %s: %w", dir, walkErr)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("snapshot of %s: %w", dir, closeErr)
	}
	return hashes, nil
}

// Changed lists, sorted, the files modified, added or removed between two snapshots.
func Changed(before, after map[string]string) []string {
	var changed []string
	for path, hash := range after {
		if before[path] != hash {
			changed = append(changed, path)
		}
	}
	for path := range before {
		if _, ok := after[path]; !ok {
			changed = append(changed, path)
		}
	}
	slices.Sort(changed)
	return slices.Compact(changed)
}
