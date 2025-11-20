package sqlite

import (
	"testing"

	"github.com/djeebus/ftpsync/lib"
	"github.com/stretchr/testify/require"
)

func TestHappyPath(t *testing.T) {
	db, err := New(":memory:")
	require.NoError(t, err)

	path1 := "/one/path1"
	path2 := "/two/path2"

	allPaths := lib.NewSet()
	allPaths.Set(path1)
	allPaths.Set(path2)

	onlyPath1 := lib.NewSet()
	onlyPath1.Set(path1)

	onlyPath2 := lib.NewSet()
	onlyPath2.Set(path2)

	emptySet := lib.NewSet()

	err = db.Record(path1)
	require.NoError(t, err)

	err = db.Record(path2)
	require.NoError(t, err)

	ok, err := db.Exists(path1)
	require.NoError(t, err)
	require.True(t, ok)

	files, err := db.GetAllFiles("/one")
	require.NoError(t, err)
	require.Equal(t, onlyPath1, files)

	err = db.Delete(path1)
	require.NoError(t, err)

	ok, err = db.Exists(path1)
	require.NoError(t, err)
	require.False(t, ok)

	files, err = db.GetAllFiles("/two")
	require.NoError(t, err)
	require.Equal(t, onlyPath2, files)

	err = db.Record(path2)
	require.NoError(t, err)

	err = db.Delete(path2)
	require.NoError(t, err)

	files, err = db.GetAllFiles("/two")
	require.NoError(t, err)
	require.Equal(t, emptySet, files)
}
