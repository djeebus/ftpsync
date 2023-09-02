package config

import (
	"os"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var expectedMaxConfig = Config{
	Database:    "test-database",
	Source:      "test-source",
	Precheck:    "test-precheck",
	Destination: "test-destination",
	RootDir:     "test-root-dir",
	DirMode:     0o421,
	FileMode:    0o422,
	LogFormat:   "test-test",
	LogLevel:    logrus.DebugLevel,
	DirUserID:   30,
	DirGroupID:  31,
	FileUserID:  32,
	FileGroupID: 33,
}

func TestMarshalConfigFromEnv(t *testing.T) {
	t.Setenv("FTPSYNC_DATABASE", expectedMaxConfig.Database)
	t.Setenv("FTPSYNC_DESTINATION", expectedMaxConfig.Destination)
	t.Setenv("FTPSYNC_DIR_MODE", "0421")
	t.Setenv("FTPSYNC_FILE_MODE", "0422")
	t.Setenv("FTPSYNC_LOG_FORMAT", expectedMaxConfig.LogFormat)
	t.Setenv("FTPSYNC_LOG_LEVEL", "debug")
	t.Setenv("FTPSYNC_ROOT_DIR", "test-root-dir")
	t.Setenv("FTPSYNC_SOURCE", expectedMaxConfig.Source)
	t.Setenv("FTPSYNC_PRECHECK", expectedMaxConfig.Precheck)
	t.Setenv("FTPSYNC_DIR_USER_ID", "30")
	t.Setenv("FTPSYNC_DIR_GROUP_ID", "31")
	t.Setenv("FTPSYNC_FILE_USER_ID", "32")
	t.Setenv("FTPSYNC_FILE_GROUP_ID", "33")

	flags := pflag.NewFlagSet("a flag set", pflag.PanicOnError)
	SetupFlags(flags)

	c, err := ReadConfig(flags)
	require.NoError(t, err)

	assert.Equal(t, expectedMaxConfig, c)
}

func TestMarshalConfigFromYaml(t *testing.T) {
	yamlFile := `
database: test-database
destination: test-destination
dir-group-id: 31
dir-mode: 0421
dir-user-id: 30
file-group-id: 33
file-mode: 0422
file-user-id: 32
log-format: test-test
log-level: debug
precheck: test-precheck
root-dir: test-root-dir
source: test-source
`
	tempdir := os.TempDir()
	tempfile, err := os.CreateTemp(tempdir, "config*.yaml")
	require.NoError(t, err)

	_, err = tempfile.Write([]byte(yamlFile))
	require.NoError(t, err)

	err = tempfile.Close()
	require.NoError(t, err)

	t.Setenv("FTPSYNC_CONFIG", tempfile.Name())

	flags := pflag.NewFlagSet("a flag set", pflag.PanicOnError)
	SetupFlags(flags)
	c, err := ReadConfig(flags)
	require.NoError(t, err)

	assert.Equal(t, expectedMaxConfig, c)
}
