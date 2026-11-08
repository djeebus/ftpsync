package localfs

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/djeebus/ftpsync/lib/config"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/djeebus/ftpsync/lib"
)

func New(config config.Config, logger logrus.FieldLogger) (*LocalFS, error) {
	return &LocalFS{
		logger:      logger,
		root:        config.Destination,
		dirMode:     config.DirMode,
		dirGroupID:  config.DirGroupID,
		dirUserID:   config.DirUserID,
		fileMode:    config.FileMode,
		fileUserID:  config.FileUserID,
		fileGroupID: config.FileGroupID,
	}, nil
}

var _ lib.Destination = new(LocalFS)

type LocalFS struct {
	dirMode  fs.FileMode
	fileMode fs.FileMode

	fileUserID  config.UserID
	fileGroupID config.GroupID

	dirUserID  config.UserID
	dirGroupID config.GroupID

	logger logrus.FieldLogger
	root   string
}

func (l *LocalFS) getFsys() fs.FS {
	// so much trimming required to get FS to work
	root := strings.TrimRight(l.root, "/")
	fsys := os.DirFS(root)
	return fsys
}

func (l *LocalFS) GetAllFiles(rootPath string) (*lib.SizeSet, error) {
	files := lib.NewSizeSet()
	if err := l.walkFiles(rootPath, func(path string, d fs.DirEntry) error {
		if strings.HasPrefix(d.Name(), partialPrefix) {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return errors.Wrapf(err, "failed to read info %s", path)
		}

		files.Set("/"+path, info.Size())
		return nil
	}); err != nil {
		return nil, err
	}

	return files, nil
}

// GetAllPartials returns the paths of the files that have partial downloads
func (l *LocalFS) GetAllPartials(rootPath string) (*lib.Set, error) {
	partials := lib.NewSet()
	if err := l.walkFiles(rootPath, func(path string, d fs.DirEntry) error {
		name, ok := strings.CutPrefix(d.Name(), partialPrefix)
		if !ok {
			return nil
		}

		partials.Set("/" + filepath.Join(filepath.Dir(path), name))
		return nil
	}); err != nil {
		return nil, err
	}

	return partials, nil
}

func (l *LocalFS) walkFiles(rootPath string, fn func(path string, d fs.DirEntry) error) error {
	fsys := l.getFsys()

	rootPath = strings.TrimLeft(rootPath, "/")
	rootPath = strings.TrimRight(rootPath, "/")

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

		return fn(path, d)
	}); err != nil {
		return errors.Wrapf(err, "failed to walk [%s, %s]", l.root, rootPath)
	}

	return nil
}

func (l *LocalFS) toLocalPath(path string) string {
	path = strings.TrimLeft(path, "/")
	path = filepath.Join(l.root, path)
	return path
}

func (l *LocalFS) Exists(path string) (bool, error) {
	path = l.toLocalPath(path)

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, errors.Wrap(err, "failed to check local path")
	} else {
		return true, nil
	}
}

func (l *LocalFS) Delete(path string) error {
	path = l.toLocalPath(path)
	if err := os.Remove(path); err != nil {
		return errors.Wrap(err, "failed to delete file")
	}

	return nil
}

func (l *LocalFS) DeletePartial(path string) error {
	path = toPartialPath(l.toLocalPath(path))
	if err := os.Remove(path); err != nil {
		return errors.Wrap(err, "failed to delete partial file")
	}

	return nil
}

// partialPrefix marks in-progress downloads as hidden files, so they can be resumed after a failure
const partialPrefix = ".ftpsync-partial."

func toPartialPath(path string) string {
	return filepath.Join(filepath.Dir(path), partialPrefix+filepath.Base(path))
}

func (l *LocalFS) PartialSize(path string) (int64, error) {
	info, err := os.Stat(toPartialPath(l.toLocalPath(path)))
	if os.IsNotExist(err) {
		return 0, nil
	} else if err != nil {
		return 0, errors.Wrap(err, "failed to check partial file")
	}

	return info.Size(), nil
}

func (l *LocalFS) WriteAt(path string, fp io.Reader, offset, size int64) (int64, error) {
	var err error
	path = l.toLocalPath(path)
	dirname := filepath.Dir(path)

	newDirs := missingDirs(dirname)
	if err = os.MkdirAll(dirname, l.dirMode); err != nil {
		return 0, errors.Wrap(err, "failed to create directory")
	}
	for _, dir := range newDirs {
		if err = chown(dir, int(l.dirUserID), int(l.dirGroupID)); err != nil {
			return 0, errors.Wrap(err, "failed to chown directory")
		}
	}

	partialPath := toPartialPath(path)
	partial, err := os.OpenFile(partialPath, os.O_WRONLY|os.O_CREATE, l.fileMode)
	if err != nil {
		return 0, errors.Wrap(err, "failed to open partial file")
	}

	// drop anything past offset, so a short or restarted read never leaves stale bytes behind
	if err = partial.Truncate(offset); err != nil {
		l.safelyClose(partial)
		return 0, errors.Wrap(err, "failed to truncate partial file")
	}
	if _, err = partial.Seek(offset, io.SeekStart); err != nil {
		l.safelyClose(partial)
		return 0, errors.Wrap(err, "failed to seek partial file")
	}

	written, err := io.Copy(partial, fp)
	total := offset + written
	if err != nil {
		l.safelyClose(partial)
		return total, fmt.Errorf("failed to write file to disk (wrote %d bytes): %w", written, err)
	}

	if err = partial.Close(); err != nil {
		return total, errors.Wrap(err, "failed to close partial file")
	}

	if total != size {
		return total, fmt.Errorf("download ended at %d bytes, expected %d", total, size)
	}

	if err = os.Rename(partialPath, path); err != nil {
		return total, errors.Wrap(err, "failed to rename partial file to final destination")
	}

	if err = os.Chmod(path, l.fileMode); err != nil {
		return total, errors.Wrap(err, "failed to set the mode")
	}

	if err = chown(path, int(l.fileUserID), int(l.fileGroupID)); err != nil {
		return total, errors.Wrap(err, "failed to set file owner")
	}

	return total, nil
}

// missingDirs returns dir and each of its ancestors that do not exist yet, outermost first.
func missingDirs(dir string) []string {
	var dirs []string
	for {
		if _, err := os.Stat(dir); err == nil {
			break
		}
		dirs = append([]string{dir}, dirs...)

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return dirs
}

// chown sets whichever of uid and gid are configured; 0 means unset.
func chown(path string, uid, gid int) error {
	if uid == 0 && gid == 0 {
		return nil
	}
	if uid == 0 {
		uid = -1
	}
	if gid == 0 {
		gid = -1
	}
	return os.Chown(path, uid, gid)
}

func (l *LocalFS) cleanDirectories(path string) (isDeleted bool, err error) {
	var (
		hasChildren bool
		wasDeleted  bool
	)

	entries, err := os.ReadDir(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}

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

func (l *LocalFS) CleanDirectories(path string) error {
	path = l.toLocalPath(path)

	_, err := l.cleanDirectories(path)

	return err
}

func (l *LocalFS) safelyClose(fp *os.File) {
	if err := fp.Close(); err != nil {
		l.logger.
			WithField("path", fp.Name()).
			WithError(err).
			Error("failed to close file")
	}
}
