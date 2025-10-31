package config

import (
	"os"
	"os/user"
	"reflect"
	"regexp"
	"strconv"
	"sync"

	"github.com/caarlos0/env/v11"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
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

func ReadConfig() (Config, error) {
	return env.ParseAsWithOptions[Config](env.Options{
		FuncMap: map[reflect.Type]env.ParserFunc{
			reflect.TypeOf(logrus.Level(0)): func(v string) (interface{}, error) {
				return logrus.ParseLevel(v)
			},
			reflect.TypeOf(UserID(0)): func(v string) (interface{}, error) {
				return parseUser(v)
			},
			reflect.TypeOf(GroupID(0)): func(v string) (interface{}, error) {
				return parseGroup(v)
			},
			reflect.TypeOf(os.FileMode(0)): func(v string) (interface{}, error) {
				mode64, err := strconv.ParseInt(v, 8, 32)
				if err != nil {
					return 0, errors.Wrap(err, "failed to parse mode")
				}
				return os.FileMode(mode64), nil
			},
		},
	})
}
