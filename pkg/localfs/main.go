package localfs

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/djeebus/ftpsync/pkg"
	"github.com/pkg/errors"
)

func BuildDestination(dst string) (pkg.Destination, error) {
	return &LocalDestination{dst}, nil
}

type LocalDestination struct {
	root string
}

func (l *LocalDestination) toLocalPath(path string) string {
	path = strings.TrimLeft(path, "/")
	path = filepath.Join(l.root, path)
	return path
}

func (l *LocalDestination) Exists(path string) (bool, error) {
	path = l.toLocalPath(path)

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, errors.Wrap(err, "failed to check local path")
	} else {
		return true, nil
	}
}

func (l *LocalDestination) Write(path string, fp io.ReadCloser) (int64, error) {
	defer func() {
		if err := fp.Close(); err != nil {
			fmt.Printf("failed to close reader for %s: %v", path, err)
		}
	}()

	var err error
	path = l.toLocalPath(path)
	dirname := filepath.Dir(path)

	if err = os.MkdirAll(dirname, 0777); err != nil {
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

	if err := os.Rename(temppath.Name(), path); err != nil {
		return 0, errors.Wrap(err, "failed to rename temp file to final destination")
	}

	return size, nil
}

var _ pkg.Destination = new(LocalDestination)
