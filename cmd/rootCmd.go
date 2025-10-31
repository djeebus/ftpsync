package cmd

import (
	"context"
	"net/url"
	"os/signal"
	"syscall"
	"time"

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

	if precheckURL != nil {
		switch precheckURL.Scheme {
		case "deluge", "deluges":
			if precheck, err = deluge.New(log, precheckURL, config.RootDir); err != nil {
				return errors.Wrap(err, "failed to create deluge precheck")
			}
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

	if config.Repeat != 0 {
		ctx := context.Background()
		ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
		defer cancel()

		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(config.Repeat):
				if err = processor.Process(config.RootDir); err != nil {
					log.WithError(err).Warning("failed to process")
				}
			}
		}
	}

	return nil
}
