package cmd

import (
	"strings"

	"github.com/djeebus/ftpsync/lib/config"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var RootCmd cobra.Command

func init() {
	RootCmd.RunE = func(cmd *cobra.Command, args []string) error {
		config, err := config.ReadConfig(&RootCmd)
		if err != nil {
			return errors.Wrap(err, "failed to get config")
		}

		if config.Source == "" {
			return errors.New("must define a source")
		}
		if config.Destination == "" {
			return errors.New("must define a precheck")
		}

		log := logrus.New()
		log.SetLevel(config.LogLevel)
		if strings.ToLower(config.LogFormat) == "json" {
			log.SetFormatter(&logrus.JSONFormatter{})
		}

		if err = doSync(config, log); err != nil {
			log.WithError(err).Fatal("sync failed")
		}

		return nil
	}
}
