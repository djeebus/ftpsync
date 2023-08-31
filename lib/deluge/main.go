package deluge

import (
	"context"
	"fmt"
	"net/url"
	"path/filepath"

	"github.com/djeebus/ftpsync/lib"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golift.io/deluge"
)

func New(log logrus.FieldLogger, url *url.URL, rootDir string) (*Deluge, error) {
	client, err := createClient(url)
	if err != nil {
		return nil, err
	}

	fileStatuses, err := getXfers(log, client)
	if err != nil {
		return nil, err
	}

	return &Deluge{client, fileStatuses, log, rootDir}, nil
}

func getXfers(log logrus.FieldLogger, client *deluge.Deluge) (map[string]bool, error) {
	var err error

	if err = client.Login(); err != nil {
		return nil, errors.Wrap(err, "failed to log in")
	}

	transfers, err := client.GetXfers()
	if err != nil {
		return nil, errors.Wrap(err, "failed to list transfers")
	}

	fileStatuses := make(map[string]bool)
	for _, xfer := range transfers {
		for idx := range xfer.Files {
			file := xfer.Files[idx]
			progress := xfer.FileProgress[idx]
			isReady := progress == 1
			fileStatuses[file.Path] = isReady
			log.WithFields(logrus.Fields{
				"path":     file.Path,
				"isReady":  isReady,
				"progress": progress,
			}).Debug("deluge file info")
		}
	}

	return fileStatuses, nil
}

func createClient(url *url.URL) (*deluge.Deluge, error) {
	switch url.Scheme {
	case "deluge":
		url.Scheme = "http"
	case "deluges":
		url.Scheme = "https"
	default:
		return nil, fmt.Errorf("unknown schema: %s", url.Scheme)
	}

	pass, _ := url.User.Password()
	config := deluge.Config{
		URL:      url.String(),
		Password: pass,
	}

	client, err := deluge.New(context.TODO(), &config)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create client")
	}
	return client, nil
}

type Deluge struct {
	client       *deluge.Deluge
	fileStatuses map[string]bool
	log          logrus.FieldLogger
	rootDir      string
}

func (d *Deluge) IsFileReady(path string) (bool, error) {
	path, err := filepath.Rel(d.rootDir, path)
	if err != nil {
		return false, err
	}

	isReady, exists := d.fileStatuses[path]
	if !exists {
		d.log.WithFields(logrus.Fields{
			"path": path,
		}).Warning("path does not exist in deluge")
		return false, nil
	}

	return isReady, nil
}

var _ lib.Precheck = new(Deluge)
