package cmd

import (
	"context"
	"fmt"
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

func doSync(ctx context.Context, config config.Config, log logrus.FieldLogger) error {
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
	case "ftp", "ftps", "ftps-implicit":
		if source, err = ftp.New(srcURL); err != nil {
			return errors.Wrap(err, "failed to build ftp source")
		}
	case "filebrowser":
		if source, err = filebrowser.New(srcURL, log); err != nil {
			return errors.Wrap(err, "failed to build filebrowser source")
		}
	default:
		return fmt.Errorf("unknown source scheme: %q", srcURL.Scheme)
	}
	defer source.Close()

	if precheckURL != nil {
		switch precheckURL.Scheme {
		case "deluge", "deluges":
			if precheck, err = deluge.New(ctx, log, precheckURL, config.RootDir); err != nil {
				return errors.Wrap(err, "failed to create deluge precheck")
			}
			defer precheck.Close()
		default:
			return fmt.Errorf("unknown precheck scheme: %q", precheckURL.Scheme)
		}
	}

	if database, err = sqlite.New(config.Database); err != nil {
		return errors.Wrap(err, "failed to build database")
	}
	defer database.Close()

	if destination, err = localfs.New(config, log); err != nil {
		return errors.Wrap(err, "failed to build destination")
	}

	processor := lib.BuildProcessor(source, database, precheck, destination, log)

	return processor.Process(ctx, config.RootDir)
}
