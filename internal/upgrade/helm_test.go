package upgrade

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHelmChartState_FormattedMessage(t *testing.T) {
	const chart = "metal3"

	state := ChartStateUnknown
	assert.Equal(t, "State of chart metal3 is unknown", state.FormattedMessage(chart))

	state = ChartStateNotInstalled
	assert.Equal(t, "Chart metal3 is not installed", state.FormattedMessage(chart))

	state = ChartStateVersionAlreadyInstalled
	assert.Equal(t, "Specified version of chart metal3 is already installed", state.FormattedMessage(chart))

	state = ChartStateInProgress
	assert.Equal(t, "Chart metal3 upgrade is in progress", state.FormattedMessage(chart))

	state = ChartStateFailed
	assert.Equal(t, "Chart metal3 upgrade failed", state.FormattedMessage(chart))

	state = ChartStateSucceeded
	assert.Equal(t, "Chart metal3 upgrade succeeded", state.FormattedMessage(chart))

	state = 99 // non-existing
	assert.Equal(t, "", state.FormattedMessage(chart))
}
