package cmd

import (
	"context"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/djeebus/ftpsync/lib/config"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func RootCmd() error {
	ctx := context.Background()
	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	cfg, err := config.ReadConfig()
	if err != nil {
		return errors.Wrap(err, "failed to get config")
	}

	log := logrus.New()
	log.SetLevel(cfg.LogLevel)
	if strings.ToLower(cfg.LogFormat) == "json" {
		log.SetFormatter(&logrus.JSONFormatter{})
	}

	if cfg.Repeat != 0 {
		if err = doSync(ctx, cfg, log); err != nil && ctx.Err() == nil {
			log.WithError(err).Warning("failed to process")
		}

		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(cfg.Repeat):
				if err = doSync(ctx, cfg, log); err != nil && ctx.Err() == nil {
					log.WithError(err).Warning("failed to process")
				}
			}
		}
	}

	return doSync(ctx, cfg, log)
}
