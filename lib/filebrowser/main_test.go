package filebrowser

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

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

func newTestServer(t *testing.T, content string) (*FileBrowser, *int) {
	logins := 0
	validToken := ""

	mux := http.NewServeMux()
	mux.HandleFunc("/api/login", func(w http.ResponseWriter, r *http.Request) {
		logins++
		validToken = fmt.Sprintf("token-%d", logins)
		_, _ = w.Write([]byte(validToken))
	})
	mux.HandleFunc("/api/raw/", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Auth") != validToken {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		http.ServeContent(w, r, "file", time.Time{}, strings.NewReader(content))
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	u, err := url.Parse(server.URL)
	require.NoError(t, err)

	f, err := New(u, logrus.New())
	require.NoError(t, err)
	f.url.Scheme = "http"

	return f, &logins
}

func TestFileBrowser_ReadOffset(t *testing.T) {
	f, _ := newTestServer(t, "hello world")
	require.NoError(t, f.login())

	body, offset, err := f.ReadAt("/file", 6)
	require.NoError(t, err)
	defer body.Close()

	data, err := io.ReadAll(body)
	require.NoError(t, err)
	assert.Equal(t, int64(6), offset)
	assert.Equal(t, "world", string(data))
}

func TestFileBrowser_ReadRelogin(t *testing.T) {
	f, logins := newTestServer(t, "hello world")
	require.NoError(t, f.login())

	// server rotates the token, so ours is rejected
	f.authCookie = "expired"

	body, offset, err := f.ReadAt("/file", 0)
	require.NoError(t, err)
	defer body.Close()

	data, err := io.ReadAll(body)
	require.NoError(t, err)
	assert.Equal(t, int64(0), offset)
	assert.Equal(t, "hello world", string(data))
	assert.Equal(t, 2, *logins)
}

func TestFileBrowser_ReadRefreshesOldToken(t *testing.T) {
	f, logins := newTestServer(t, "hello world")
	require.NoError(t, f.login())
	f.authTime = time.Now().Add(-2 * maxTokenAge)

	body, _, err := f.ReadAt("/file", 0)
	require.NoError(t, err)
	body.Close()

	assert.Equal(t, 2, *logins)
}
