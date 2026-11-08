package ftp

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jlaffaye/ftp"
	"github.com/pkg/errors"

	"github.com/djeebus/ftpsync/lib"
)

// New connects to url. The scheme picks the transport: "ftp" is plain text,
// "ftps" upgrades with AUTH TLS (explicit), and "ftps-implicit" speaks TLS from
// the start. Pass ?insecure=true to skip certificate verification.
func New(url *url.URL) (lib.Source, error) {
	insecure := false
	if value := url.Query().Get("insecure"); value != "" {
		var err error
		if insecure, err = strconv.ParseBool(value); err != nil {
			return nil, errors.Wrap(err, "failed to parse insecure")
		}
	}
	tlsConfig := &tls.Config{
		ServerName:         url.Hostname(),
		InsecureSkipVerify: insecure,
	}

	var opts []ftp.DialOption
	switch url.Scheme {
	case "ftp":
	case "ftps":
		opts = append(opts, ftp.DialWithExplicitTLS(tlsConfig))
	case "ftps-implicit":
		opts = append(opts, ftp.DialWithTLS(tlsConfig))
	default:
		return nil, fmt.Errorf("unknown ftp scheme: %q", url.Scheme)
	}

	conn, err := ftp.Dial(url.Host, opts...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to dial ftp server")
	}

	password, _ := url.User.Password()
	if err = conn.Login(url.User.Username(), password); err != nil {
		_ = conn.Quit()
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
	rootPath := f.toRemotePath(path)

	entries, err := f.conn.List(rootPath)
	if err != nil {
		return lib.NewListResult(), errors.Wrapf(err, "failed to walk %s", path)
	}

	return toListResult(entries)
}

// toListResult converts a directory listing into names relative to that directory.
func toListResult(entries []*ftp.Entry) (lib.ListResult, error) {
	result := lib.NewListResult()

	for _, entry := range entries {
		if entry.Name == "." || entry.Name == ".." {
			continue
		}

		switch entry.Type {
		case ftp.EntryTypeFolder:
			result.Folders = append(result.Folders, entry.Name)
		case ftp.EntryTypeFile:
			result.Files[entry.Name] = int64(entry.Size)
		case ftp.EntryTypeLink:
			// TODO: implement link handling
			continue
		default:
			return result, fmt.Errorf("unknown file type for %s: %d", entry.Name, entry.Type)
		}
	}

	return result, nil
}

func (f *source) ReadAt(path string, offset int64) (io.ReadCloser, int64, error) {
	path = f.toRemotePath(path)

	e, err := f.conn.RetrFrom(path, uint64(offset))
	if err != nil {
		return nil, 0, errors.Wrap(err, "failed to download file")
	}

	return e, offset, nil
}

func (f *source) Close() error {
	return f.conn.Quit()
}

var _ lib.Source = new(source)
