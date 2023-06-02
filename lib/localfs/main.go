package localfs

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"

	"github.com/djeebus/ftpsync/lib"
)

func New(dst string, dirMode, fileMode fs.FileMode) (lib.Destination, error) {
	return &destination{
		root:     dst,
		dirMode:  dirMode,
		fileMode: fileMode,
	}, nil
}

type destination struct {
	dirMode  fs.FileMode
	fileMode fs.FileMode
	root     string
}

func (l *destination) GetAllFiles(rootPath string) (*lib.Set, error) {
	// so much trimming required to get FS to work
	root := strings.TrimRight(l.root, "/")
	rootPath = strings.TrimLeft(rootPath, "/")
	rootPath = strings.TrimRight(rootPath, "/")

	fsys := os.DirFS(root)

	files := lib.NewSet()
	if err := fs.WalkDir(fsys, rootPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return errors.Wrapf(err, "error at %s", path)
		}

		if d.IsDir() {
			return nil
		}

		files.Set("/" + path)
		return nil
	}); err != nil {
		return nil, errors.Wrapf(err, "failed to walk [%s, %s]", l.root, rootPath)
	}

	return files, nil
}

func (l *destination) toLocalPath(path string) string {
	path = strings.TrimLeft(path, "/")
	path = filepath.Join(l.root, path)
	return path
}

func (l *destination) Exists(path string) (bool, error) {
	path = l.toLocalPath(path)

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, errors.Wrap(err, "failed to check local path")
	} else {
		return true, nil
	}
}

func (l *destination) Delete(path string) error {
	path = l.toLocalPath(path)
	if err := os.Remove(path); err != nil {
		return errors.Wrap(err, "failed to delete file")
	}

	return nil
}

func (l *destination) Write(path string, fp io.ReadCloser) (int64, error) {
	defer func() {
		if err := fp.Close(); err != nil {
			fmt.Printf("failed to close reader for %s: %v", path, err)
		}
	}()

	var err error
	path = l.toLocalPath(path)
	dirname := filepath.Dir(path)

	if err = os.MkdirAll(dirname, l.dirMode); err != nil {
		return 0, errors.Wrap(err, "failed to create directory")
	}

	temppath, err := os.CreateTemp(dirname, "temp")
	if err != nil {
		return 0, errors.Wrap(err, "failed to create temp file")
	}

	size, err := io.Copy(temppath, fp)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to write %s", path)
	}
	if err = temppath.Close(); err != nil {
		return 0, errors.Wrap(err, "failed to close temp file")
	}

	if err = os.Rename(temppath.Name(), path); err != nil {
		return 0, errors.Wrap(err, "failed to rename temp file to final destination")
	}

	if err = os.Chmod(path, l.fileMode); err != nil {
		return 0, errors.Wrap(err, "failed to set the mode")
	}

	return size, nil
}

var _ lib.Destination = new(destination)
