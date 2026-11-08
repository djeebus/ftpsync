package localfs

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/djeebus/ftpsync/lib/config"
)

func TestIgnoreNoSuchFile(t *testing.T) {
	var d LocalFS

	_, err := d.cleanDirectories("/a/b/c/d")
	require.NoError(t, err)
}

type failingReader struct {
	data []byte
}

func (r *failingReader) Read(p []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, io.ErrUnexpectedEOF
	}
	n := copy(p, r.data)
	r.data = r.data[n:]
	return n, nil
}

func TestWriteResume(t *testing.T) {
	root := t.TempDir()
	d, err := New(config.Config{Destination: root, DirMode: 0o755, FileMode: 0o644}, logrus.New())
	require.NoError(t, err)

	// first attempt dies partway through
	size, err := d.WriteAt("/a/file.txt", &failingReader{data: []byte("hello ")}, 0)
	require.ErrorIs(t, err, io.ErrUnexpectedEOF)
	assert.Equal(t, int64(6), size)

	partial, err := d.PartialSize("/a/file.txt")
	require.NoError(t, err)
	assert.Equal(t, int64(6), partial)
	assert.FileExists(t, filepath.Join(root, "a", ".ftpsync-partial.file.txt"))

	// partial downloads are not reported as local files
	files, err := d.GetAllFiles("/a")
	require.NoError(t, err)
	assert.Equal(t, 0, files.Len())

	// resume from where it left off
	size, err = d.WriteAt("/a/file.txt", strings.NewReader("world"), partial)
	require.NoError(t, err)
	assert.Equal(t, int64(11), size)

	data, err := os.ReadFile(filepath.Join(root, "a", "file.txt"))
	require.NoError(t, err)
	assert.Equal(t, "hello world", string(data))

	partial, err = d.PartialSize("/a/file.txt")
	require.NoError(t, err)
	assert.Equal(t, int64(0), partial)
}

func TestWriteRestartTruncates(t *testing.T) {
	root := t.TempDir()
	d, err := New(config.Config{Destination: root, DirMode: 0o755, FileMode: 0o644}, logrus.New())
	require.NoError(t, err)

	_, err = d.WriteAt("/file.txt", &failingReader{data: []byte("stale data")}, 0)
	require.Error(t, err)

	_, err = d.WriteAt("/file.txt", strings.NewReader("new"), 0)
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(root, "file.txt"))
	require.NoError(t, err)
	assert.Equal(t, "new", string(data))
}

func TestGetAllPartials(t *testing.T) {
	root := t.TempDir()
	d, err := New(config.Config{Destination: root, DirMode: 0o755, FileMode: 0o644}, logrus.New())
	require.NoError(t, err)

	_, err = d.WriteAt("/a/b/file.txt", &failingReader{data: []byte("partial")}, 0)
	require.Error(t, err)

	partials, err := d.GetAllPartials("/a")
	require.NoError(t, err)
	assert.Equal(t, []string{"/a/b/file.txt"}, partials.ToList())

	require.NoError(t, d.DeletePartial("/a/b/file.txt"))
	assert.NoFileExists(t, filepath.Join(root, "a", "b", ".ftpsync-partial.file.txt"))
}
