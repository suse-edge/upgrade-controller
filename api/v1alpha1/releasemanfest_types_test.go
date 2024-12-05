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

func TestConvertContainersSliceToMap(t *testing.T) {
	expContainer1 := "foo"
	expContainerImage1 := "bar"
	expContainer2 := "bar"
	expContainerImage2 := "baz"

	component := CoreComponent{
		Containers: []CoreComponentContainer{
			{
				Name:  expContainer1,
				Image: expContainerImage1,
			},
			{
				Name:  expContainer2,
				Image: expContainerImage2,
			},
		},
	}

	m := component.ConvertContainerSliceToMap()
	assert.Equal(t, 2, len(m))

	v, ok := m[expContainer1]
	assert.True(t, ok)
	assert.Equal(t, expContainerImage1, v)

	v, ok = m[expContainer2]
	assert.True(t, ok)
	assert.Equal(t, expContainerImage2, v)

	component = CoreComponent{}
	m = component.ConvertContainerSliceToMap()
	assert.Empty(t, m)
}
