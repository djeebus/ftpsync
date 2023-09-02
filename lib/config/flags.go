package config

import "github.com/spf13/pflag"

func SetupFlags(flags *pflag.FlagSet) {
	flags.String("source", "", "location from which to pull files")
	flags.String("destination", "", "location to push files to")
	flags.String("root-dir", "/", "remote path to sync")
	flags.String("database", "ftpsync.db", "path to database")
	flags.String("dir-mode", "0777", "mode for directories")
	flags.String("file-mode", "0666", "mode for files")
	flags.String("dir-user-id", "", "user for directories")
	flags.String("file-user-id", "", "user for files")
	flags.String("dir-group-id", "", "group for directorie")
	flags.String("file-group-id", "", "group for files")
	flags.String("log-level", "warning", "log level")
	flags.String("log-format", "text", "log format")
	flags.String("precheck", "", "precheck")
}
