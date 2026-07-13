package input

import (
	"archive/zip"
	"errors"
	"io"
	"os"
	pathpkg "path"
	"path/filepath"
	"strings"
)

const MaxCompressedSize int64 = 100 << 20
const MaxUncompressedSize uint64 = 100 << 20
const MaxArchiveEntries = 10000

func Open(path string) (io.ReadCloser, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("input must be a regular file")
	}
	if info.Size() > MaxCompressedSize {
		return nil, errors.New("input exceeds 100 MiB limit")
	}
	if strings.EqualFold(filepath.Ext(path), ".zip") {
		z, err := zip.OpenReader(path)
		if err != nil {
			return nil, err
		}
		if len(z.File) > MaxArchiveEntries {
			_ = z.Close()
			return nil, errors.New("ZIP contains too many members")
		}
		var match *zip.File
		for _, f := range z.File {
			cleanName := pathpkg.Clean(strings.ReplaceAll(f.Name, "\\", "/"))
			unsafePath := cleanName == ".." || strings.HasPrefix(cleanName, "../") || strings.HasPrefix(cleanName, "/")
			if f.FileInfo().Mode()&os.ModeSymlink != 0 || unsafePath || f.Flags&0x1 != 0 {
				_ = z.Close()
				return nil, errors.New("unsafe ZIP member")
			}
			if f.UncompressedSize64 > MaxUncompressedSize {
				_ = z.Close()
				return nil, errors.New("ZIP member exceeds 100 MiB limit")
			}
			if !f.FileInfo().IsDir() && strings.EqualFold(filepath.Ext(f.Name), ".xml") {
				if match != nil {
					_ = z.Close()
					return nil, errors.New("ZIP contains multiple XML files")
				}
				match = f
			}
		}
		if match == nil {
			_ = z.Close()
			return nil, errors.New("ZIP contains no XML file")
		}
		r, err := match.Open()
		if err != nil {
			_ = z.Close()
			return nil, err
		}
		return &zipReadCloser{Reader: r, closers: []io.Closer{r, z}}, nil
	}
	return os.Open(path)
}

type zipReadCloser struct {
	io.Reader
	closers []io.Closer
}

func (z *zipReadCloser) Close() error {
	var first error
	for _, c := range z.closers {
		if err := c.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}
