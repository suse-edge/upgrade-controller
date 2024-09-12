package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestArch_Short(t *testing.T) {
	assert.Equal(t, "amd64", ArchTypeX86.Short())
	assert.Equal(t, "arm64", ArchTypeARM.Short())
	assert.PanicsWithValue(t, "unknown arch: abc", func() {
		arch := Arch("abc")
		arch.Short()
	})
}

func TestSupportedArchitectures(t *testing.T) {
	archs := []Arch{ArchTypeARM, ArchTypeX86}

	supported := SupportedArchitectures(archs)
	require.Len(t, supported, 4)

	assert.Contains(t, supported, "x86_64")
	assert.Contains(t, supported, "aarch64")
	assert.Contains(t, supported, "amd64")
	assert.Contains(t, supported, "arm64")
}
