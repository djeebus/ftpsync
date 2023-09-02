package config

import (
	"os"
	"os/user"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

type lookupName func(name string) (string, error)

var allNumbers = regexp.MustCompile(`^\d+$`)

var (
	currentUserOnce sync.Once
	currentUser     *user.User
)

func getCurrentUser() *user.User {
	currentUserOnce.Do(func() {
		u, err := user.Current()
		if err != nil {
			panic(err)
		}
		currentUser = u

	})

	return currentUser
}

func parseId(id string, lookupName lookupName) (int, error) {
	var err error

	if !allNumbers.MatchString(id) {
		id, err = lookupName(id)
	}

	if err != nil {
		return 0, errors.Wrap(err, "failed to lookup id")
	}

	return strconv.Atoi(id)
}

func parseUser(userID string) (int, error) {
	if userID == "" {
		userID = getCurrentUser().Uid
	}

	return parseId(userID, func(name string) (string, error) {
		if u, err := user.Lookup(name); err != nil {
			return "", errors.Wrapf(err, "failed to lookup '%s' user", name)
		} else {
			return u.Uid, nil
		}
	})
}

func parseGroup(groupId string) (int, error) {
	if groupId == "" {
		groupId = getCurrentUser().Gid
	}

	return parseId(groupId, func(name string) (string, error) {
		if g, err := user.LookupGroup(name); err != nil {
			return "", errors.Wrapf(err, "failed to lookup '%s' group", name)
		} else {
			return g.Gid, nil
		}

	})
}

func decodeLogrusLevel(f, t reflect.Type, data interface{}) (interface{}, error) {
	if f.Kind() != reflect.String {
		return data, nil
	}

	if t == reflect.TypeOf(UserID(0)) {
		return parseUser(data.(string))
	}

	if t == reflect.TypeOf(GroupID(0)) {
		return parseGroup(data.(string))
	}

	if t == reflect.TypeOf(logrus.Level(0)) {
		return logrus.ParseLevel(data.(string))
	}

	if t == reflect.TypeOf(os.FileMode(0)) {
		mode64, err := strconv.ParseInt(data.(string), 8, 32)
		if err != nil {
			return 0, errors.Wrap(err, "failed to parse mode")
		}
		return os.FileMode(mode64), nil
	}

	return data, nil
}

func ReadConfig(flags *pflag.FlagSet) (Config, error) {
	var (
		err    error
		config Config
		v      = viper.New()
	)

	if err = v.BindPFlags(flags); err != nil {
		return config, errors.Wrap(err, "failed to bind flags")
	}

	v.SetEnvPrefix("FTPSYNC")
	v.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	v.AutomaticEnv()

	if path := os.Getenv("FTPSYNC_CONFIG"); path != "" {
		// beware: goofy stuff to make it work w/ viper
		configDir, configName := filepath.Split(path)
		v.AddConfigPath(configDir)

		baseName := filepath.Base(configName)
		idx := strings.LastIndex(baseName, ".")
		v.SetConfigName(baseName[:idx])

		if err = v.ReadInConfig(); err != nil {
			return config, errors.Wrap(err, "failed to read in config")
		}
	}

	err = v.Unmarshal(&config, viper.DecodeHook(decodeLogrusLevel))
	if err != nil {
		return config, errors.Wrap(err, "failed to unmarshal config")
	}

	return config, nil
}
