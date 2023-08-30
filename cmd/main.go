package cmd

import (
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	rootDir      string
	dbLocation   string
	dirModeStr   string
	fileModeStr  string
	dirUserStr   string
	dirGroupStr  string
	fileGroupStr string
	fileUserStr  string
	logLevelStr  string
	logFormat    string
	precheckStr  string
)

var RootCmd = cobra.Command{
	RunE: func(cmd *cobra.Command, args []string) error {
		var src, pre, dst string
		switch len(args) {
		case 2:
			src = args[0]
			dst = args[1]
		case 3:
			src = args[0]
			pre = args[1]
			dst = args[2]
		default:
			return errors.New("usage: ftpsync SRC [PRECHECK] DST")
		}

		var (
			err      error
			logLevel logrus.Level
		)

		if logLevel, err = parseLogLevel(logLevelStr); err != nil {
			return errors.Wrap(err, "failed to parse log level")
		}

		log := logrus.New()
		log.SetLevel(logLevel)
		if strings.ToLower(logFormat) == "json" {
			log.SetFormatter(&logrus.JSONFormatter{})
		}

		if err = doSync(src, pre, dst, log); err != nil {
			log.WithError(err).Fatal("sync failed")
		}

		return nil
	},
}

func init() {
	RootCmd.PersistentFlags().StringVar(&rootDir, "root", "/", "remote path to sync")
	RootCmd.PersistentFlags().StringVar(&dbLocation, "database", "ftpsync.db", "path to database")
	RootCmd.PersistentFlags().StringVar(&dirModeStr, "dir-mode", "0777", "mode for directories")
	RootCmd.PersistentFlags().StringVar(&fileModeStr, "file-mode", "0666", "mode for files")
	RootCmd.PersistentFlags().StringVar(&dirUserStr, "dir-user", "", "user for directories")
	RootCmd.PersistentFlags().StringVar(&fileUserStr, "file-user", "", "user for files")
	RootCmd.PersistentFlags().StringVar(&dirGroupStr, "dir-group", "", "group for directorie")
	RootCmd.PersistentFlags().StringVar(&fileGroupStr, "file-group", "", "group for files")
	RootCmd.PersistentFlags().StringVar(&logLevelStr, "log-level", "warning", "log level")
	RootCmd.PersistentFlags().StringVar(&logFormat, "log-format", "text", "log format")
	RootCmd.PersistentFlags().StringVar(&precheckStr, "precheck", "", "precheck")
}
