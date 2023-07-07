package ftp

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/jlaffaye/ftp"
	"github.com/pkg/errors"

	"github.com/djeebus/ftpsync/lib"
)

func New(url *url.URL) (lib.Source, error) {
	var opts = ftp.DialOption{}
	switch url.Scheme {
	case "sftp":
		opts = ftp.DialWithTLS(&tls.Config{
			ServerName: url.Host,
		})
	case "ftps":
		opts = ftp.DialWithExplicitTLS(&tls.Config{
			//ServerName: url.Host,
			InsecureSkipVerify: true,
		})
	}

	conn, err := ftp.Dial(url.Host, opts)
	if err != nil {
		return nil, errors.Wrap(err, "failed to dial ftp server")
	}

	password, _ := url.User.Password()
	if err = conn.Login(url.User.Username(), password); err != nil {
		return nil, errors.Wrap(err, "failed to login")
	}

	return &source{conn: conn, root: url.Path}, nil
}

type source struct {
	root string
	conn *ftp.ServerConn
}

func (f *source) GetAllFiles(path string) (*lib.SizeSet, error) {
	return lib.WalkLister(f, path)
}

func (f *source) toRemotePath(path string) string {
	path = strings.TrimLeft(path, "/")
	path = filepath.Join(f.root, path)
	return path
}

func (f *source) List(path string) (lib.ListResult, error) {
	result := lib.NewListResult()

	rootPath := f.toRemotePath(path)

	entries, err := f.conn.List(rootPath)
	if err != nil {
		return result, errors.Wrapf(err, "failed to walk %s", path)
	}

	for _, entry := range entries {
		entryPath := filepath.Join(path, entry.Name)

		switch entry.Type {
		case ftp.EntryTypeFolder:
			result.Folders = append(result.Folders, entryPath)
		case ftp.EntryTypeFile:
			result.Files[entryPath] = int64(entry.Size)
		case ftp.EntryTypeLink:
			// TODO: implement link handling
			continue
		default:
			return result, fmt.Errorf("unknown file type for %s: %s", entry.Name, entry.Type.String())
		}
	}

	return result, nil
}

func (f *source) Read(path string) (io.ReadCloser, error) {
	path = f.toRemotePath(path)

	e, err := f.conn.Retr(path)
	if err != nil {
		return nil, errors.Wrap(err, "failed to download file")
	}

	return e, nil
}

var _ lib.Source = new(source)
