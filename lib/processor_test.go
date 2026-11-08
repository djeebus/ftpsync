package lib_test

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/djeebus/ftpsync/lib"
	"github.com/djeebus/ftpsync/lib/config"
	"github.com/djeebus/ftpsync/lib/localfs"
	"github.com/djeebus/ftpsync/lib/sqlite"
)

type fakeSource struct {
	files map[string]string
}

func (f *fakeSource) ReadAt(path string, offset int64) (io.ReadCloser, int64, error) {
	return io.NopCloser(strings.NewReader(f.files[path][offset:])), offset, nil
}

func (f *fakeSource) GetAllFiles(string) (*lib.SizeSet, error) {
	files := lib.NewSizeSet()
	for path, content := range f.files {
		files.Set(path, int64(len(content)))
	}
	return files, nil
}

func (f *fakeSource) Close() error { return nil }

type notReady struct{}

func (notReady) IsFileReady(string) (bool, error) { return false, nil }
func (notReady) Close() error                     { return nil }

func TestProcessCleansPartials(t *testing.T) {
	root := t.TempDir()
	local, err := localfs.New(config.Config{Destination: root, DirMode: 0o755, FileMode: 0o644}, logrus.New())
	require.NoError(t, err)

	db, err := sqlite.New(filepath.Join(t.TempDir(), "db.sqlite"))
	require.NoError(t, err)

	writePartial := func(name string) {
		require.NoError(t, os.MkdirAll(filepath.Join(root, "d"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(root, "d", ".ftpsync-partial."+name), []byte("x"), 0o644))
	}

	// already downloaded, leftover partial
	require.NoError(t, os.MkdirAll(filepath.Join(root, "d"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "d", "done.txt"), []byte("done"), 0o644))
	writePartial("done.txt")
	// no longer on the remote
	writePartial("gone.txt")
	// still pending, the precheck holds it back
	writePartial("pending.txt")

	src := &fakeSource{files: map[string]string{
		"/d/done.txt":    "done",
		"/d/pending.txt": "pending",
	}}

	p := lib.BuildProcessor(src, db, notReady{}, local, logrus.New())
	require.NoError(t, p.Process("/d"))

	partials, err := local.GetAllPartials("/d")
	require.NoError(t, err)
	assert.Equal(t, []string{"/d/pending.txt"}, partials.ToList())
}
