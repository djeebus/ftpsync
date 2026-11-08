package lib

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFmtDoesNotOverflowMarkers(t *testing.T) {
	assert.Equal(t, "2.00KB", fmtSize(2048))
	assert.Equal(t, "2048.00TB", fmtSize(2048<<40))
	assert.Equal(t, "+InfTB/s", fmtSpeed(1, 0))
	assert.Equal(t, "2.00KB/s", fmtSpeed(2048, time.Second))
	assert.NotPanics(t, func() { fmtSpeed(math.MaxInt64, time.Nanosecond) })
}
