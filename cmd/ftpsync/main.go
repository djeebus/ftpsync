package main

import (
	"fmt"
	"os"
	"strconv"

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

		if err := doSync(args[0], args[1]); err != nil {
			fmt.Printf("sync failed: \n\n%v\n", err)
			os.Exit(1)
		}

		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&rootDir, "root", "/", "remote path to sync")
	rootCmd.PersistentFlags().StringVar(&dbLocation, "database", "ftpsync.db", "path to database")
	rootCmd.PersistentFlags().StringVar(&dirModeStr, "dir-mode", "0777", "mode for directories")
	rootCmd.PersistentFlags().StringVar(&fileModeStr, "file-mode", "0666", "mode for files")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println("error: ", err)
		os.Exit(1)
	}
}
