package cacheparity

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io/fs"
	"os/exec"
	"strconv"
	"time"
)

// archiveSpec describes an archive fixture. The test builds the bytes at run
// time and hands both implementations the same file, so that no binary is
// committed.
type archiveSpec struct {
	// Format is tar, tgz, tbz2 (needs the bzip2 binary to compress) or zip.
	Format string        `json:"format"`
	Files  []archiveFile `json:"files"`
	// Truncate drops that many bytes from the end of the archive.
	Truncate int `json:"truncate,omitempty"`
}

// archiveFile is a member of an archive.
type archiveFile struct {
	Name    string `json:"name"`
	Content string `json:"content,omitempty"`
	// Mode is octal. The default is 0644 for files and 0755 for directories.
	Mode string `json:"mode,omitempty"`
	// Type is file (default), dir, symlink or hardlink.
	Type string `json:"type,omitempty"`
	// Link is the target of a symlink or a hardlink.
	Link string `json:"link,omitempty"`
}

var fixedTime = time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)

func (f archiveFile) mode(dflt fs.FileMode) fs.FileMode {
	if f.Mode == "" {
		return dflt
	}
	m, err := strconv.ParseUint(f.Mode, 8, 32)
	if err != nil {
		panic(fmt.Sprintf("bad mode %q in fixture %q", f.Mode, f.Name))
	}
	return fs.FileMode(m)
}

// build returns the bytes of the archive.
func (a archiveSpec) build() ([]byte, error) {
	var data []byte
	var err error
	switch a.Format {
	case "tar", "tgz", "tbz2":
		data, err = a.buildTar()
	case "zip":
		data, err = a.buildZip()
	default:
		return nil, fmt.Errorf("unknown archive format %q", a.Format)
	}
	if err != nil {
		return nil, err
	}
	if a.Truncate > 0 {
		data = data[:len(data)-a.Truncate]
	}
	return data, nil
}

func (a archiveSpec) buildTar() ([]byte, error) {
	var raw bytes.Buffer
	tw := tar.NewWriter(&raw)
	for _, f := range a.Files {
		h := &tar.Header{Name: f.Name, ModTime: fixedTime, Format: tar.FormatPAX}
		switch f.Type {
		case "", "file":
			h.Typeflag, h.Mode, h.Size = tar.TypeReg, int64(f.mode(0o644)), int64(len(f.Content))
		case "dir":
			h.Typeflag, h.Mode = tar.TypeDir, int64(f.mode(0o755))
		case "symlink":
			h.Typeflag, h.Mode, h.Linkname = tar.TypeSymlink, int64(f.mode(0o777)), f.Link
		case "hardlink":
			h.Typeflag, h.Mode, h.Linkname = tar.TypeLink, int64(f.mode(0o644)), f.Link
		default:
			return nil, fmt.Errorf("unknown member type %q", f.Type)
		}
		if err := tw.WriteHeader(h); err != nil {
			return nil, err
		}
		if h.Typeflag == tar.TypeReg {
			if _, err := tw.Write([]byte(f.Content)); err != nil {
				return nil, err
			}
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	switch a.Format {
	case "tgz":
		var out bytes.Buffer
		zw := gzip.NewWriter(&out)
		zw.ModTime = fixedTime
		if _, err := zw.Write(raw.Bytes()); err != nil {
			return nil, err
		}
		if err := zw.Close(); err != nil {
			return nil, err
		}
		return out.Bytes(), nil
	case "tbz2":
		// the standard library only decompresses bzip2
		cmd := exec.Command("bzip2", "-c")
		cmd.Stdin = &raw
		return cmd.Output()
	}
	return raw.Bytes(), nil
}

func (a archiveSpec) buildZip() ([]byte, error) {
	var out bytes.Buffer
	zw := zip.NewWriter(&out)
	for _, f := range a.Files {
		h := &zip.FileHeader{Name: f.Name, Method: zip.Deflate, Modified: fixedTime}
		var content []byte
		switch f.Type {
		case "", "file":
			h.SetMode(f.mode(0o644))
			content = []byte(f.Content)
		case "dir":
			h.SetMode(fs.ModeDir | f.mode(0o755))
			h.Method = zip.Store
		case "symlink":
			h.SetMode(fs.ModeSymlink | f.mode(0o777))
			h.Method = zip.Store
			content = []byte(f.Link)
		default:
			return nil, fmt.Errorf("unknown member type %q", f.Type)
		}
		w, err := zw.CreateHeader(h)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(content); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// resolveArchives replaces every archive description of the scenarios by the
// base64 content of the built archive, which is all the reference driver and
// the Go side need to materialise it.
func resolveArchives(scenarios []scenario) error {
	for _, sc := range scenarios {
		for _, group := range [][]entry{sc.Src, sc.Cache, sc.Temp} {
			for i := range group {
				if group[i].Archive == nil {
					continue
				}
				data, err := group[i].Archive.build()
				if err != nil {
					return fmt.Errorf("%s: %s: %w", sc.Name, group[i].Path, err)
				}
				group[i].Base64 = base64.StdEncoding.EncodeToString(data)
				group[i].Archive = nil
			}
		}
	}
	return nil
}
