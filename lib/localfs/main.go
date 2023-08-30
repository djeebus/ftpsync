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

func New(dst string, dirMode, fileMode fs.FileMode, fileUserID, fileGroupID int) (lib.Destination, error) {
	return &destination{
		root:        dst,
		dirMode:     dirMode,
		fileMode:    fileMode,
		fileUserID:  fileUserID,
		fileGroupID: fileGroupID,
	}, nil
}

type destination struct {
	dirMode  fs.FileMode
	fileMode fs.FileMode

	fileUserID  int
	fileGroupID int

	root string
}

func (l *destination) getFsys() fs.FS {
	// so much trimming required to get FS to work
	root := strings.TrimRight(l.root, "/")
	fsys := os.DirFS(root)
	return fsys
}

func (l *destination) GetAllFiles(rootPath string) (*lib.SizeSet, error) {
	fsys := l.getFsys()

	rootPath = strings.TrimLeft(rootPath, "/")
	rootPath = strings.TrimRight(rootPath, "/")

	files := lib.NewSizeSet()
	if err := fs.WalkDir(fsys, rootPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}

			return errors.Wrapf(err, "error at %s", path)
		}

		if d.IsDir() {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return errors.Wrapf(err, "failed to read info %s", path)
		}

		files.Set("/"+path, info.Size())
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

	if err = os.Chown(path, l.fileUserID, l.fileGroupID); err != nil {
		return 0, errors.Wrap(err, "failed to set file owner")
	}

	return size, nil
}

func (l *destination) cleanDirectories(path string) (isDeleted bool, err error) {
	var (
		hasChildren bool
		wasDeleted  bool
	)

	entries, err := os.ReadDir(path)
	if err != nil {
		return false, errors.Wrapf(err, "failed to read %s", path)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			dirPath := filepath.Join(path, entry.Name())
			wasDeleted, err = l.cleanDirectories(dirPath)
			if err != nil {
				return false, errors.Wrapf(err, "failed to clean %s", dirPath)
			}
			if wasDeleted {
				continue
			}
		}

		hasChildren = true
	}

	if hasChildren {
		return false, nil
	}

	if err = os.Remove(path); err != nil {
		return false, errors.Wrapf(err, "failed to remove %s", path)
	}

	return true, nil
}

func (l *destination) CleanDirectories(path string) error {
	path = l.toLocalPath(path)

	_, err := l.cleanDirectories(path)

	return err
}

var _ lib.Destination = new(destination)
