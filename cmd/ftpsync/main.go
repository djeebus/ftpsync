package main

import (
	"fmt"
	"net/url"
	"os"
	"strconv"

	"github.com/djeebus/ftpsync/pkg"
	"github.com/djeebus/ftpsync/pkg/filebrowser"
	"github.com/djeebus/ftpsync/pkg/ftp"
	"github.com/djeebus/ftpsync/pkg/localfs"
	"github.com/djeebus/ftpsync/pkg/sqlite"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var (
	rootDir     string
	dbLocation  string
	dirModeStr  string
	fileModeStr string
)

func parseFsMode(mode string) (os.FileMode, error) {
	mode64, err := strconv.ParseInt(mode, 8, 32)
	if err != nil {
		return 0, errors.Wrap(err, "failed to parse mode")
	}
	return os.FileMode(mode64), nil
}

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

			dirMode  os.FileMode
			fileMode os.FileMode
		)

		url, err = url.Parse(src)
		if err != nil {
			return errors.Wrap(err, "fail to parse url")
		}

		if fileMode, err = parseFsMode(fileModeStr); err != nil {
			return errors.Wrap(err, "failed to parse file mode")
		}
		if dirMode, err = parseFsMode(dirModeStr); err != nil {
			return errors.Wrap(err, "failed to parse dir mode")
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

		if database, err = sqlite.BuildDatabase(dbLocation); err != nil {
			return errors.Wrap(err, "failed to build database")
		}
		if destination, err = localfs.BuildDestination(dst, dirMode, fileMode); err != nil {
			return errors.Wrap(err, "failed to build destination")
		}

		processor := pkg.BuildProcessor(source, database, destination)
		return processor.Process(rootDir)
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&rootDir, "root", "/", "remote path to sync")
	rootCmd.PersistentFlags().StringVar(&dbLocation, "database", "ftpsync.db", "path to database")
	rootCmd.PersistentFlags().StringVar(&fileModeStr, "dir-mode", "0777", "mode for directories")
	rootCmd.PersistentFlags().StringVar(&fileModeStr, "file-mode", "0666", "mode for files")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println("error: ", err)
	}
}
