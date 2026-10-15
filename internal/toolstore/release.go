package toolstore

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
)

const (
	// maxBinary bounds what is extracted from an archive, so a hostile archive cannot fill the disk.
	maxBinary    = 1 << 30
	ownerExecute = 0o100
)

// ReleasePlatform is the only platform tools.lock pins release archives for.
const ReleasePlatform = "linux/amd64"

// release says where a [release] tool's linux/amd64 archive is and which member is the binary.
type release struct {
	url    func(version string) string
	zipped bool
	member func(version string) string
}

func constant(s string) func(string) string { return func(string) string { return s } }

var releases = map[string]release{
	"golangci-lint": {
		url: func(v string) string {
			return "https://github.com/golangci/golangci-lint/releases/download/v" + v + "/golangci-lint-" + v + "-linux-amd64.tar.gz"
		},
		member: func(v string) string { return "golangci-lint-" + v + "-linux-amd64/golangci-lint" },
	},
	"ast-grep": {
		url: func(v string) string {
			return "https://github.com/ast-grep/ast-grep/releases/download/" + v + "/app-x86_64-unknown-linux-gnu.zip"
		},
		zipped: true,
		member: constant("ast-grep"),
	},
	"gitleaks": {
		url: func(v string) string {
			return "https://github.com/gitleaks/gitleaks/releases/download/v" + v + "/gitleaks_" + v + "_linux_x64.tar.gz"
		},
		member: constant("gitleaks"),
	},
	"actionlint": {
		url: func(v string) string {
			return "https://github.com/rhysd/actionlint/releases/download/v" + v + "/actionlint_" + v + "_linux_amd64.tar.gz"
		},
		member: constant("actionlint"),
	},
}

// extract writes the archive member to dest, executable by its owner. Only that one member is
// read; no path from the archive is ever used to write.
func extract(archive []byte, zipped bool, member, dest string) error {
	var found archived
	var err error
	if zipped {
		found, err = zipMember(archive, member)
	} else {
		found, err = tarMember(archive, member)
	}
	if err != nil {
		return err
	}
	if err = os.WriteFile(dest, found.data, found.mode.Perm()|ownerExecute); err != nil {
		return fmt.Errorf("writing %s: %w", dest, err)
	}
	return nil
}

// archived is one member read out of an archive.
type archived struct {
	data []byte
	mode fs.FileMode
}

// readBounded reads the whole member, failing when it is larger than maxBinary.
func readBounded(member string, src io.Reader, mode fs.FileMode) (archived, error) {
	data, err := io.ReadAll(io.LimitReader(src, maxBinary+1))
	if err != nil {
		return archived{}, fmt.Errorf("extracting %s: %w", member, err)
	}
	if len(data) > maxBinary {
		return archived{}, fmt.Errorf("%s in the archive is larger than %d bytes", member, int64(maxBinary))
	}
	return archived{data: data, mode: mode}, nil
}

func tarMember(archive []byte, member string) (archived, error) {
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return archived{}, fmt.Errorf("opening the archive: %w", err)
	}
	tr := tar.NewReader(gz)
	for {
		header, nextErr := tr.Next()
		if errors.Is(nextErr, io.EOF) {
			return archived{}, fmt.Errorf("the archive has no member %s", member)
		}
		if nextErr != nil {
			return archived{}, fmt.Errorf("reading the archive: %w", nextErr)
		}
		if header.Typeflag == tar.TypeReg && path.Clean(header.Name) == member {
			return readBounded(member, tr, header.FileInfo().Mode())
		}
	}
}

func zipMember(archive []byte, member string) (archived, error) {
	zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return archived{}, fmt.Errorf("opening the archive: %w", err)
	}
	for _, f := range zr.File {
		if path.Clean(f.Name) != member {
			continue
		}
		rc, openErr := f.Open()
		if openErr != nil {
			return archived{}, fmt.Errorf("opening %s in the archive: %w", member, openErr)
		}
		found, readErr := readBounded(member, rc, f.Mode())
		if closeErr := rc.Close(); readErr == nil && closeErr != nil {
			readErr = fmt.Errorf("closing %s in the archive: %w", member, closeErr)
		}
		return found, readErr
	}
	return archived{}, fmt.Errorf("the archive has no member %s", member)
}
