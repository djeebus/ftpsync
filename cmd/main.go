package cmd

import (
	"strings"

	"github.com/djeebus/ftpsync/lib/config"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var RootCmd = cobra.Command{
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.ReadConfig(cmd.PersistentFlags())
		if err != nil {
			return errors.Wrap(err, "failed to get config")
		}

		if cfg.Source == "" {
			return errors.New("must define a source")
		}
		if cfg.Destination == "" {
			return errors.New("must define a precheck")
		}

		log := logrus.New()
		log.SetLevel(cfg.LogLevel)
		if strings.ToLower(cfg.LogFormat) == "json" {
			log.SetFormatter(&logrus.JSONFormatter{})
		}

		if err = doSync(cfg, log); err != nil {
			log.WithError(err).Fatal("sync failed")
		}

		return nil
	},
}

func init() {
	config.SetupFlags(RootCmd.PersistentFlags())
}
