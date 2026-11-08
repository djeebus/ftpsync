package ftp

import (
	"testing"

	"github.com/jlaffaye/ftp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/djeebus/ftpsync/lib"
)

type fakeLister map[string][]*ftp.Entry

func (f fakeLister) List(path string) (lib.ListResult, error) {
	return toListResult(f[path])
}

func TestWalkUsesRelativeNames(t *testing.T) {
	lister := fakeLister{
		"/d": {
			{Name: ".", Type: ftp.EntryTypeFolder},
			{Name: "..", Type: ftp.EntryTypeFolder},
			{Name: "top.txt", Type: ftp.EntryTypeFile, Size: 3},
			{Name: "sub", Type: ftp.EntryTypeFolder},
			{Name: "link", Type: ftp.EntryTypeLink},
		},
		"/d/sub": {
			{Name: "nested.txt", Type: ftp.EntryTypeFile, Size: 6},
		},
	}

	files, err := lib.WalkLister(lister, "/d")
	require.NoError(t, err)
	assert.Equal(t, 2, files.Len())

	size, ok := files.Get("/d/top.txt")
	assert.True(t, ok)
	assert.Equal(t, int64(3), size)

	size, ok = files.Get("/d/sub/nested.txt")
	assert.True(t, ok)
	assert.Equal(t, int64(6), size)
}

func TestUnknownEntryType(t *testing.T) {
	_, err := toListResult([]*ftp.Entry{{Name: "x", Type: ftp.EntryType(99)}})
	require.Error(t, err)
}
