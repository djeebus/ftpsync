package cmd

import (
	"net/url"

	"github.com/djeebus/ftpsync/lib/config"
	"github.com/djeebus/ftpsync/lib/deluge"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/djeebus/ftpsync/lib"
	"github.com/djeebus/ftpsync/lib/filebrowser"
	"github.com/djeebus/ftpsync/lib/ftp"
	"github.com/djeebus/ftpsync/lib/localfs"
	"github.com/djeebus/ftpsync/lib/sqlite"
)

func doSync(config config.Config, log logrus.FieldLogger) error {
	var (
		err         error
		precheckURL *url.URL
		srcURL      *url.URL
		source      lib.Source
		precheck    lib.Precheck
		database    lib.Database
		destination lib.Destination
	)

	if config.Precheck != "" {
		precheckURL, err = url.Parse(config.Precheck)
		if err != nil {
			return errors.Wrap(err, "failed to parse precheck")
		}
	}

	srcURL, err = url.Parse(config.Source)
	if err != nil {
		return errors.Wrap(err, "fail to parse url")
	}

	switch srcURL.Scheme {
	case "ftp", "ftps", "sftp":
		if source, err = ftp.New(srcURL); err != nil {
			return errors.Wrap(err, "failed to build ftp source")
		}
	case "filebrowser":
		if source, err = filebrowser.New(srcURL); err != nil {
			return errors.Wrap(err, "failed to build filebrowser source")
		}
	default:
		return errors.New("unknown source")
	}
	defer source.Close()

	if precheckURL != nil {
		switch precheckURL.Scheme {
		case "deluge", "deluges":
			if precheck, err = deluge.New(log, precheckURL, config.RootDir); err != nil {
				return errors.Wrap(err, "failed to create deluge precheck")
			}
			defer precheck.Close()
		}
	}

	if database, err = sqlite.New(config.Database); err != nil {
		return errors.Wrap(err, "failed to build database")
	}
	if destination, err = localfs.New(config); err != nil {
		return errors.Wrap(err, "failed to build destination")
	}

	processor := lib.BuildProcessor(source, database, precheck, destination, log)

	if err := processor.Process(config.RootDir); err != nil {
		return err
	}

	return nil
}
