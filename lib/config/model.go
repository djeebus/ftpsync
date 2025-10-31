package config

import (
	"os"
	"time"

	"github.com/sirupsen/logrus"
)

type UserID int
type GroupID int

type Config struct {
	Database    string `env:"DATABASE,required"`
	Source      string `env:"SOURCE,required"`
	Precheck    string `env:"PRECHECK"`
	Destination string `env:"DESTINATION,required"`

	Repeat time.Duration `env:"REPEAT,required"`

	RootDir string `env:"ROOT_DIR,required"`

	DirMode  os.FileMode `env:"DIR_MODE"`
	FileMode os.FileMode `env:"FILE_MODE"`

	LogFormat string       `env:"LOG_FORMAT"`
	LogLevel  logrus.Level `env:"LOG_LEVEL"`

	DirUserID  UserID  `env:"DIR_USER_ID"`
	DirGroupID GroupID `env:"DIR_GROUP_ID"`

	FileUserID  UserID  `env:"FILE_USER_ID"`
	FileGroupID GroupID `env:"FILE_GROUP_ID"`
}
