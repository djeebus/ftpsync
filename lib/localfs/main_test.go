package localfs

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIgnoreNoSuchFile(t *testing.T) {
	var d destination

	_, err := d.cleanDirectories("/a/b/c/d")
	require.NoError(t, err)
}
