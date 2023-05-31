package main

import (
	"fmt"
	"net/url"

	"github.com/djeebus/ftpsync/pkg"
	"github.com/djeebus/ftpsync/pkg/filebrowser"
	"github.com/djeebus/ftpsync/pkg/ftp"
	"github.com/djeebus/ftpsync/pkg/localfs"
	"github.com/djeebus/ftpsync/pkg/sqlite"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var rootDir string

var rootCmd = cobra.Command{
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 2 {
			return errors.New("usage: ftpsync SRC DST")
		}

		var (
			src = args[0]
			dst = args[1]

			err         error
			url         *url.URL
			source      pkg.Source
			database    pkg.Database
			destination pkg.Destination
		)

		url, err = url.Parse(src)
		if err != nil {
			return errors.Wrap(err, "fail to parse url")
		}

		switch url.Scheme {
		case "ftp", "ftps", "sftp":
			if source, err = ftp.BuildSource(url); err != nil {
				return errors.Wrap(err, "failed to build ftp source")
			}
		case "filebrowser":
			if source, err = filebrowser.BuildSource(url); err != nil {
				return errors.Wrap(err, "failed to build filebrowser source")
			}
		}

		if database, err = sqlite.BuildDatabase(); err != nil {
			return errors.Wrap(err, "failed to build database")
		}
		if destination, err = localfs.BuildDestination(dst); err != nil {
			return errors.Wrap(err, "failed to build destination")
		}

		processor := pkg.BuildProcessor(source, database, destination)
		return processor.Process(rootDir)
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&rootDir, "root", "/", "remote path to sync")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println("error: ", err)
	}
}
