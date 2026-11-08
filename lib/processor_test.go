package lib_test

import (
	"context"
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
	require.NoError(t, p.Process(context.Background(), "/d"))

	partials, err := local.GetAllPartials("/d")
	require.NoError(t, err)
	assert.Equal(t, []string{"/d/pending.txt"}, partials.ToList())
}

func TestProcessStopsWhenCancelled(t *testing.T) {
	root := t.TempDir()
	local, err := localfs.New(config.Config{Destination: root, DirMode: 0o755, FileMode: 0o644}, logrus.New())
	require.NoError(t, err)

	db, err := sqlite.New(filepath.Join(t.TempDir(), "db.sqlite"))
	require.NoError(t, err)

	src := &fakeSource{files: map[string]string{"/d/file.txt": "data"}}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	p := lib.BuildProcessor(src, db, nil, local, logrus.New())
	require.ErrorIs(t, p.Process(ctx, "/d"), context.Canceled)
	assert.NoFileExists(t, filepath.Join(root, "d", "file.txt"))
}

// blockingSource sends a few bytes, cancels, then blocks until the reader is closed
type blockingSource struct {
	cancel context.CancelFunc
}

type blockingReader struct {
	cancel context.CancelFunc
	sent   bool
	closed chan struct{}
}

func (r *blockingReader) Read(p []byte) (int, error) {
	if !r.sent {
		r.sent = true
		return copy(p, "abc"), nil
	}
	r.cancel()
	<-r.closed
	return 0, io.ErrClosedPipe
}

func (r *blockingReader) Close() error {
	close(r.closed)
	return nil
}

func (b *blockingSource) ReadAt(string, int64) (io.ReadCloser, int64, error) {
	return &blockingReader{cancel: b.cancel, closed: make(chan struct{})}, 0, nil
}

func (b *blockingSource) GetAllFiles(string) (*lib.SizeSet, error) {
	files := lib.NewSizeSet()
	files.Set("/d/big.bin", 100)
	return files, nil
}

func (b *blockingSource) Close() error { return nil }

func TestProcessAbortsDownloadWhenCancelled(t *testing.T) {
	root := t.TempDir()
	local, err := localfs.New(config.Config{Destination: root, DirMode: 0o755, FileMode: 0o644}, logrus.New())
	require.NoError(t, err)

	db, err := sqlite.New(filepath.Join(t.TempDir(), "db.sqlite"))
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	p := lib.BuildProcessor(&blockingSource{cancel: cancel}, db, nil, local, logrus.New())
	require.NoError(t, p.Process(ctx, "/d"))

	partial, err := local.PartialSize("/d/big.bin")
	require.NoError(t, err)
	assert.Equal(t, int64(3), partial)
}
