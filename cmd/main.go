package cmd

import (
	"fmt"
	"os"

	"github.com/pkg/errors"
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
)

var RootCmd = cobra.Command{
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
	RootCmd.PersistentFlags().StringVar(&rootDir, "root", "/", "remote path to sync")
	RootCmd.PersistentFlags().StringVar(&dbLocation, "database", "ftpsync.db", "path to database")
	RootCmd.PersistentFlags().StringVar(&dirModeStr, "dir-mode", "0777", "mode for directories")
	RootCmd.PersistentFlags().StringVar(&fileModeStr, "file-mode", "0666", "mode for files")
	RootCmd.PersistentFlags().StringVar(&dirUserStr, "dir-user", "", "user for directories")
	RootCmd.PersistentFlags().StringVar(&fileUserStr, "file-user", "", "user for files")
	RootCmd.PersistentFlags().StringVar(&dirGroupStr, "dir-group", "", "group for directorie")
	RootCmd.PersistentFlags().StringVar(&fileGroupStr, "file-group", "", "group for files")
}
