package filebrowser

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"

	"github.com/djeebus/ftpsync/lib"
)

func New(url *url.URL) (lib.Source, error) {
	// pull data off url
	username := url.User.Username()
	password, _ := url.User.Password()

	// clean url
	url.Scheme = "https"
	url.User = nil
	url.RawQuery = ""

	fbs := new(source)
	fbs.url = url

	return &source{
		url:      url,
		username: username,
		password: password,
	}, nil
}

type source struct {
	url    *url.URL
	client http.Client

	username, password string
	authCookie         string
}

func (f *source) toUrl(path string) string {
	path = strings.TrimLeft(path, "/")
	newURL := f.url.JoinPath(path)
	return newURL.String()
}

func (f *source) login() error {
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
	return nil
}

func (f *source) GetAllFiles(path string) (*lib.SizeSet, error) {
	if err := f.login(); err != nil {
		return nil, fmt.Errorf("failed to login: %w", err)
	}

	return lib.WalkLister(f, path)
}

func (f *source) List(path string) (lib.ListResult, error) {
	result := lib.NewListResult()

	apiPath := strings.TrimLeft(path, "/")
	apiPath = filepath.Join("/api/resources", apiPath)
	apiPath = f.toUrl(apiPath)

	request, err := http.NewRequest("GET", apiPath, nil)
	if err != nil {
		return result, errors.Wrap(err, "failed to make request")
	}
	request.Header.Add("Cookie", fmt.Sprintf("auth=%s", f.authCookie))
	request.Header.Add("X-Auth", f.authCookie)

	response, err := f.client.Do(request)
	if err != nil {
		return result, errors.Wrap(err, "failed to get response")
	}
	defer response.Body.Close()

	if response.StatusCode >= 400 {
		return result, fmt.Errorf("failed to get file list: %d", response.StatusCode)
	}

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return result, errors.Wrap(err, "failed to read body")
	}
	var responseStruct struct {
		IsDir     bool `json:"isDir"`
		IsSymlink bool `json:"isSymlink"`
		Items     []struct {
			IsDir     bool   `json:"isDir"`
			IsSymlink bool   `json:"isSymlink"`
			Name      string `json:"name"`
			Path      string `json:"path"`
			Size      int64  `json:"size"`
		} `json:"items"`
		Name string `json:"name"`
		Path string `json:"path"`
	}
	if err = json.Unmarshal(responseBody, &responseStruct); err != nil {
		return result, errors.Wrap(err, "failed to unmarshal body")
	}

	for _, entry := range responseStruct.Items {
		if entry.IsDir {
			result.Folders = append(result.Folders, entry.Name)
		} else if entry.IsSymlink {
		} else {
			result.Files[entry.Name] = entry.Size
		}
	}

	return result, nil

}

func (f *source) Read(path string) (io.ReadCloser, error) {
	// https://sky.seedhost.eu/jioewjafioewaj/filebrowser/api/raw/downloads/.htaccess?auth=eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyIjp7ImlkIjoxLCJsb2NhbGUiOiJlbiIsInZpZXdNb2RlIjoibGlzdCIsInNpbmdsZUNsaWNrIjpmYWxzZSwicGVybSI6eyJhZG1pbiI6dHJ1ZSwiZXhlY3V0ZSI6dHJ1ZSwiY3JlYXRlIjp0cnVlLCJyZW5hbWUiOnRydWUsIm1vZGlmeSI6dHJ1ZSwiZGVsZXRlIjp0cnVlLCJzaGFyZSI6dHJ1ZSwiZG93bmxvYWQiOnRydWV9LCJjb21tYW5kcyI6W10sImxvY2tQYXNzd29yZCI6ZmFsc2UsImhpZGVEb3RmaWxlcyI6ZmFsc2UsImRhdGVGb3JtYXQiOmZhbHNlfSwiaXNzIjoiRmlsZSBCcm93c2VyIiwiZXhwIjoxNjg1NTQ2MzI2LCJpYXQiOjE2ODU1MzkxMjZ9.pNBHHV-EhUVF7VebdP3VRDk8nWK4fZHxUbnbsInq3rY&
	apiPath := strings.TrimLeft(path, "/")
	apiPath = filepath.Join("/api/raw", apiPath)
	apiPath = f.toUrl(apiPath)
	apiPath += "?auth=" + f.authCookie

	request, err := http.NewRequest("GET", apiPath, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to make request")
	}
	request.Header.Add("Cookie", fmt.Sprintf("auth=%s", f.authCookie))
	request.Header.Add("X-Auth", f.authCookie)

	response, err := f.client.Do(request)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get response")
	}

	if response.StatusCode >= 400 {
		response.Body.Close()
		return nil, fmt.Errorf("failed to get file: %d", response.StatusCode)
	}

	return response.Body, nil
}

var _ lib.Source = new(source)
