package upgrade

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"
)

func TestPlanIdentifierLabels(t *testing.T) {
	labels := PlanIdentifierLabels("upgrade-plan-1", "upgrade-controller-system")
	require.Len(t, labels, 2)

	assert.Equal(t, "upgrade-plan-1", labels["lifecycle.suse.com/upgrade-plan-name"])
	assert.Equal(t, "upgrade-controller-system", labels["lifecycle.suse.com/upgrade-plan-namespace"])
}

func TestGenerateSuffix(t *testing.T) {
	suffix1, err := GenerateSuffix()
	require.NoError(t, err)

	suffix2, err := GenerateSuffix()
	require.NoError(t, err)

	assert.NotEqual(t, suffix1, suffix2)
}

func TestBaseUpgradePlan_DrainEnabled(t *testing.T) {
	upgradePlan := baseUpgradePlan("upgrade-plan-1", false, nil)

	assert.Equal(t, "Plan", upgradePlan.TypeMeta.Kind)
	assert.Equal(t, "upgrade.cattle.io/v1", upgradePlan.TypeMeta.APIVersion)

	assert.Equal(t, "upgrade-plan-1", upgradePlan.ObjectMeta.Name)
	assert.Equal(t, "cattle-system", upgradePlan.ObjectMeta.Namespace)
	assert.Nil(t, upgradePlan.ObjectMeta.Labels)

	assert.Equal(t, "system-upgrade-controller", upgradePlan.Spec.ServiceAccountName)
	assert.Nil(t, upgradePlan.Spec.Drain)
}

func TestBaseUpgradePlan_DrainDisabled(t *testing.T) {
	upgradePlan := baseUpgradePlan("upgrade-plan-1", true, nil)

	assert.Equal(t, "Plan", upgradePlan.TypeMeta.Kind)
	assert.Equal(t, "upgrade.cattle.io/v1", upgradePlan.TypeMeta.APIVersion)

	assert.Equal(t, "upgrade-plan-1", upgradePlan.ObjectMeta.Name)
	assert.Equal(t, "cattle-system", upgradePlan.ObjectMeta.Namespace)
	assert.Nil(t, upgradePlan.ObjectMeta.Labels)

	assert.Equal(t, "system-upgrade-controller", upgradePlan.Spec.ServiceAccountName)
	require.NotNil(t, upgradePlan.Spec.Drain)
	assert.True(t, upgradePlan.Spec.Drain.Force)
	assert.Equal(t, ptr.To(true), upgradePlan.Spec.Drain.DeleteEmptydirData)
	assert.Equal(t, ptr.To(true), upgradePlan.Spec.Drain.IgnoreDaemonSets)
	assert.Equal(t, ptr.To(15*time.Minute), upgradePlan.Spec.Drain.Timeout)
}
