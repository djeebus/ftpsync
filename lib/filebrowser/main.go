package filebrowser

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/djeebus/ftpsync/lib"
)

func New(url *url.URL, logger logrus.FieldLogger) (*FileBrowser, error) {
	var src FileBrowser

	src.logger = logger

	// pull data off url
	src.url = url
	src.username = url.User.Username()
	src.password, _ = url.User.Password()

	query := url.Query()
	if paths, ok := query["excluded"]; ok {
		src.excludedPatterns = paths
	}

	// clean url
	url.Scheme = "https"
	url.User = nil
	url.RawQuery = ""

	// no overall timeout, downloads can legitimately take hours
	src.client.Transport = &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   30 * time.Second,
		ResponseHeaderTimeout: time.Minute,
		IdleConnTimeout:       90 * time.Second,
	}

	return &src, nil
}

// filebrowser tokens expire after 2 hours by default, refresh well before then
const maxTokenAge = time.Hour

type FileBrowser struct {
	url    *url.URL
	client http.Client
	logger logrus.FieldLogger

	excludedPatterns   []string
	username, password string
	authCookie         string
	authTime           time.Time
}

var _ lib.Source = new(FileBrowser)

func (f *FileBrowser) toUrl(path string) string {
	path = strings.TrimLeft(path, "/")
	newURL := f.url.JoinPath(path)
	return newURL.String()
}

func (f *FileBrowser) login() error {
	path := f.toUrl("/api/login")

	requestBody := struct {
		Password string `json:"password"`
		Username string `json:"username"`
	}{
		Password: f.password,
		Username: f.username,
	}
	body, err := json.Marshal(requestBody)
	if err != nil {
		return errors.Wrap(err, "failed to marshal request")
	}
	buf := bytes.NewBuffer(body)

	request, err := http.NewRequest("POST", path, buf)
	if err != nil {
		return errors.Wrap(err, "failed to make request")
	}

	response, err := f.client.Do(request)
	if err != nil {
		return errors.Wrap(err, "failed to get response")
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return errors.Wrap(err, "failed to read body")
	}
	responseString := string(responseBody)

	if response.StatusCode >= 400 {
		return fmt.Errorf("failed to get login: %d", response.StatusCode)
	}

	f.authCookie = responseString
	f.authTime = time.Now()
	return nil
}

func (f *FileBrowser) GetAllFiles(path string) (*lib.SizeSet, error) {
	if err := f.login(); err != nil {
		return nil, fmt.Errorf("failed to login: %w", err)
	}

	return lib.WalkLister(f, path)
}

type responseItem struct {
	IsDir     bool   `json:"isDir"`
	IsSymlink bool   `json:"isSymlink"`
	Name      string `json:"name"`
	Path      string `json:"path"`
	Size      int64  `json:"size"`
}

type responseType struct {
	IsDir     bool           `json:"isDir"`
	IsSymlink bool           `json:"isSymlink"`
	Items     []responseItem `json:"items"`
	Name      string         `json:"name"`
	Path      string         `json:"path"`
}

func (f *FileBrowser) List(path string) (lib.ListResult, error) {
	result := lib.NewListResult()

	apiPath := strings.TrimLeft(path, "/")
	apiPath = filepath.Join("/api/resources", apiPath)
	apiPath = f.toUrl(apiPath)

	response, err := f.request("GET", apiPath, nil)
	if err != nil {
		return result, err
	}
	defer response.Body.Close()

	if response.StatusCode >= 400 {
		return result, fmt.Errorf("failed to get file list: %d", response.StatusCode)
	}

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return result, errors.Wrap(err, "failed to read body")
	}
	var responseStruct responseType
	if err = json.Unmarshal(responseBody, &responseStruct); err != nil {
		return result, errors.Wrap(err, "failed to unmarshal body")
	}

	for _, entry := range responseStruct.Items {
		if !f.includeEntry(entry) {
			continue
		}

		if entry.IsDir {
			result.Folders = append(result.Folders, entry.Name)
		} else if entry.IsSymlink {
		} else {
			result.Files[entry.Name] = entry.Size
		}
	}

	return result, nil

}

func (f *FileBrowser) request(method, apiPath string, headers http.Header) (*http.Response, error) {
	if time.Since(f.authTime) > maxTokenAge {
		if err := f.login(); err != nil {
			return nil, errors.Wrap(err, "failed to refresh login")
		}
	}

	response, err := f.doRequest(method, apiPath, headers)
	if err != nil {
		return nil, err
	}

	if response.StatusCode == http.StatusUnauthorized {
		response.Body.Close()
		f.logger.Info("token rejected, logging in again")
		if err = f.login(); err != nil {
			return nil, errors.Wrap(err, "failed to refresh login")
		}

		return f.doRequest(method, apiPath, headers)
	}

	return response, nil
}

func (f *FileBrowser) doRequest(method, apiPath string, headers http.Header) (*http.Response, error) {
	request, err := http.NewRequest(method, apiPath, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to make request")
	}
	for key, values := range headers {
		request.Header[key] = values
	}
	request.Header.Add("Cookie", fmt.Sprintf("auth=%s", f.authCookie))
	request.Header.Add("X-Auth", f.authCookie)

	response, err := f.client.Do(request)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get response")
	}
	return response, nil
}

func (f *FileBrowser) ReadAt(path string, offset int64) (io.ReadCloser, int64, error) {
	apiPath := strings.TrimLeft(path, "/")
	apiPath = filepath.Join("/api/raw", apiPath)
	apiPath = f.toUrl(apiPath)

	headers := http.Header{}
	if offset > 0 {
		headers.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	}

	response, err := f.request("GET", apiPath, headers)
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to make request")
	}

	if response.StatusCode >= 400 {
		response.Body.Close()
		return nil, 0, fmt.Errorf("failed to get file: %d", response.StatusCode)
	}

	if response.StatusCode != http.StatusPartialContent {
		// server ignored the range, so this is the whole file
		offset = 0
	} else if start := response.Header.Get("Content-Range"); !strings.HasPrefix(start, fmt.Sprintf("bytes %d-", offset)) {
		response.Body.Close()
		return nil, 0, fmt.Errorf("unexpected content range for offset %d: %q", offset, start)
	}

	return response.Body, offset, nil
}

func (f *FileBrowser) Close() error {
	f.client.CloseIdleConnections()
	return nil
}

func (f *FileBrowser) includeEntry(entry responseItem) bool {
	if entry.IsSymlink {
		return false
	}

	if entry.IsDir {
		return true
	}

	for _, pattern := range f.excludedPatterns {
		if ok, _ := filepath.Match(pattern, entry.Path); ok {
			f.logger.
				WithField("name", entry.Name).
				WithField("pattern", pattern).
				WithField("path", entry.Path).
				Debug("matches exclusion pattern")
			return false
		}
	}

	return true
}
