package deluge

import (
	"net/url"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getRequiredEnvVar(t *testing.T, key string) string {
	text := os.Getenv(key)
	if text == "" {
		t.Skipf("must pass %s", key)
	}
	return text
}

func TestCreation(t *testing.T) {
	delugeUrlText := getRequiredEnvVar(t, "DELUGE_URL")
	xferPath := getRequiredEnvVar(t, "DELUGE_PATH")
	isXferComplete := getRequiredEnvVar(t, "DELUGE_COMPLETE") == "1"

	delugeUrl, err := url.Parse(delugeUrlText)
	require.NoError(t, err)

	precheck, err := New(delugeUrl)
	require.NoError(t, err)

	isGood, err := precheck.IsFileReady(xferPath)
	require.NoError(t, err)
	assert.Equal(t, isXferComplete, isGood)
}
