package config

import (
	"os"

	"github.com/sirupsen/logrus"
)

type UserID int
type GroupID int

type Config struct {
	Database    string
	Source      string
	Precheck    string
	Destination string

	RootDir string `mapstructure:"root-dir"`

	DirMode  os.FileMode `mapstructure:"dir-mode"`
	FileMode os.FileMode `mapstructure:"file-mode"`

	LogFormat string       `mapstructure:"log-format"`
	LogLevel  logrus.Level `mapstructure:"log-level"`

	DirUserID  UserID  `mapstructure:"dir-user-id"`
	DirGroupID GroupID `mapstructure:"dir-group-id"`

	FileUserID  UserID  `mapstructure:"file-user-id"`
	FileGroupID GroupID `mapstructure:"file-group-id"`
}
