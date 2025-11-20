package filebrowser

import (
	"net/url"
	"os"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getRequiredEnvVar(t *testing.T, key string) string {
	text := os.Getenv(key)
	if text == "" {
		t.Skipf("must pass %s", key)
	}
	return text
}

func TestFileBrowser(t *testing.T) {
	urlText := getRequiredEnvVar(t, "FILEBROWSER_URL")
	rootDir := getRequiredEnvVar(t, "FILEBROWSER_ROOT")

	url, err := url.Parse(urlText)
	require.NoError(t, err)

	logger := logrus.New()

	f, err := New(url, logger)
	require.NoError(t, err)

	files, err := f.GetAllFiles(rootDir)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, files.Len(), 1)
}

func TestFileBrowser_includeEntry(t *testing.T) {
	testCases := map[string]struct {
		excludedGlobs []string
		input         responseItem
		shouldInclude bool
	}{
		"exclude symlinks": {
			input: responseItem{
				IsSymlink: true,
			},
			shouldInclude: false,
		},

		"file matches glob": {
			excludedGlobs: []string{"*something*"},
			input: responseItem{
				IsDir: true,
				Path:  "abc/badtext",
			},
			shouldInclude: true,
		},
		"file does not match glob": {
			excludedGlobs: []string{"*g*"},
			input: responseItem{
				Path: "abc/def",
			},
			shouldInclude: true,
		},
		"file subpath matches exact": {
			excludedGlobs: []string{"*/b/*"},
			input: responseItem{
				Path: "a/b/c",
			},
			shouldInclude: false,
		},
		"file subpath matches partial": {
			excludedGlobs: []string{"*b*"},
			input: responseItem{
				Path: "a/b/c",
			},
			shouldInclude: true,
		},
		"file exact path match": {
			excludedGlobs: []string{"a"},
			input: responseItem{
				Path: "a",
			},
			shouldInclude: false,
		},
		"file partial match with pattern": {
			excludedGlobs: []string{"*/*/*.c"},
			input: responseItem{
				Path: "a/b/abc.c",
			},
			shouldInclude: false,
		},
		"file and subdir partial match with pattern": {
			excludedGlobs: []string{"*/b/*.c"},
			input: responseItem{
				Path: "a/b/abc.c",
			},
			shouldInclude: false,
		},
		"file partial match": {
			excludedGlobs: []string{"*/*/c"},
			input: responseItem{
				Path: "a/b/c",
			},
			shouldInclude: false,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			s := &FileBrowser{
				excludedPatterns: testCase.excludedGlobs,
				logger:           logrus.New(),
			}
			actual := s.includeEntry(testCase.input)
			assert.Equal(t, testCase.shouldInclude, actual)
		})
	}
}
