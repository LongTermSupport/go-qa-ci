package stage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"slices"
	"strings"

	"github.com/LongTermSupport/go-qa-ci/internal/glob"
	"github.com/LongTermSupport/go-qa-ci/internal/root"
)

// shebangBytes bounds how much of a file is read to find its interpreter.
const shebangBytes = 256

// MatchingFiles are the files the stage is given, relative to the root, slash-separated and
// sorted: tracked or not, .gitignore honoured (inside a git work tree), .git skipped and symbolic
// links left out, limited to scope when it is not empty. Without a match the list is empty; the
// caller decides that this is a skip.
func (s Stage) MatchingFiles(rootDir string, scope []string) ([]string, error) {
	for _, pattern := range slices.Concat(s.Files, s.Exclude) {
		if err := glob.Valid(pattern); err != nil {
			return nil, fmt.Errorf("stage `%s`: %w", s.Name, err)
		}
	}
	handle, err := os.OpenRoot(rootDir)
	if err != nil {
		return nil, fmt.Errorf("open project root: %w", err)
	}
	chosen, listErr := s.choose(handle, rootDir, scope)
	if closeErr := handle.Close(); listErr == nil && closeErr != nil {
		return nil, fmt.Errorf("close project root: %w", closeErr)
	}
	if listErr != nil {
		return nil, listErr
	}
	slices.Sort(chosen)
	return chosen, nil
}

func (s Stage) choose(handle *os.Root, rootDir string, scope []string) ([]string, error) {
	candidates, err := projectFiles(handle, rootDir)
	if err != nil {
		return nil, err
	}
	var chosen []string
	for _, rel := range candidates {
		if !root.InScope(rel, scope) || matchesAny(s.Exclude, rel) {
			continue
		}
		if matchesAny(s.Files, rel) {
			chosen = append(chosen, rel)
			continue
		}
		interpreted, readErr := s.interpreted(handle, rel)
		if readErr != nil {
			return nil, readErr
		}
		if interpreted {
			chosen = append(chosen, rel)
		}
	}
	return chosen, nil
}

// matchesAny reports whether name matches one of the patterns, which are already validated.
func matchesAny(patterns []string, name string) bool {
	return slices.ContainsFunc(patterns, func(pattern string) bool {
		ok, err := glob.Match(pattern, name)
		return err == nil && ok
	})
}

// interpreted reports whether the first line of the file is a #! line naming one of the stage's
// interpreters, directly or through env.
func (s Stage) interpreted(handle *os.Root, rel string) (bool, error) {
	if len(s.Shebangs) == 0 {
		return false, nil
	}
	file, err := handle.Open(rel)
	if err != nil {
		return false, fmt.Errorf("open %s: %w", rel, err)
	}
	head := make([]byte, shebangBytes)
	n, readErr := io.ReadFull(file, head)
	closeErr := file.Close()
	if readErr != nil && !errors.Is(readErr, io.ErrUnexpectedEOF) && !errors.Is(readErr, io.EOF) {
		return false, fmt.Errorf("read %s: %w", rel, readErr)
	}
	if closeErr != nil {
		return false, fmt.Errorf("close %s: %w", rel, closeErr)
	}
	name, ok := interpreter(head[:n])
	return ok && slices.Contains(s.Shebangs, name), nil
}

// interpreter is the program a #! line names, looking through env and its options.
func interpreter(head []byte) (string, bool) {
	first := head
	if end := bytes.IndexByte(head, '\n'); end >= 0 {
		first = head[:end]
	}
	rest, found := strings.CutPrefix(string(first), "#!")
	if !found {
		return "", false
	}
	words := strings.Fields(rest)
	if len(words) == 0 {
		return "", false
	}
	program := path.Base(words[0])
	if program != "env" {
		return program, true
	}
	for _, word := range words[1:] {
		if !strings.HasPrefix(word, "-") {
			return path.Base(word), true
		}
	}
	return "", false
}

// projectFiles lists the regular files of the project: git's view (tracked and untracked, minus
// ignored) inside a work tree, every file outside the .git directory otherwise.
func projectFiles(handle *os.Root, rootDir string) ([]string, error) {
	inside, err := insideWorkTree(rootDir)
	if err != nil {
		return nil, err
	}
	if !inside {
		return walk(handle)
	}
	out, err := git(rootDir, "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}
	var files []string
	for name := range strings.SplitSeq(out, "\x00") {
		if name == "" {
			continue
		}
		// A deleted tracked file is still listed; a symbolic link is not a file to check.
		info, statErr := handle.Lstat(name)
		if errors.Is(statErr, fs.ErrNotExist) || (statErr == nil && !info.Mode().IsRegular()) {
			continue
		}
		if statErr != nil {
			return nil, fmt.Errorf("stat %s: %w", name, statErr)
		}
		files = append(files, name)
	}
	return files, nil
}

func walk(handle *os.Root) ([]string, error) {
	var files []string
	err := fs.WalkDir(handle.FS(), ".", func(name string, entry fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case entry.IsDir() && entry.Name() == ".git":
			return fs.SkipDir
		case entry.Type().IsRegular():
			files = append(files, name)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("listing the project files: %w", err)
	}
	return files, nil
}

// insideWorkTree reports whether dir is inside a git work tree. A missing git binary is an error,
// not a reason to treat ignored files as files to check.
func insideWorkTree(dir string) (bool, error) {
	cmd := exec.CommandContext(context.Background(), "git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = dir
	out, err := cmd.Output()
	var exitErr *exec.ExitError
	switch {
	case err == nil:
		return strings.TrimSpace(string(out)) == "true", nil
	case errors.As(err, &exitErr):
		return false, nil
	default:
		return false, fmt.Errorf("git: %w; it is needed to honour .gitignore when choosing files", err)
	}
}

func git(dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(context.Background(), "git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return string(out), nil
}
