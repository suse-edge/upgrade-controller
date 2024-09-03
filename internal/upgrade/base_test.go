package upgrade

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"
)

func TestPlanIdentifierAnnotations(t *testing.T) {
	annotations := PlanIdentifierAnnotations("upgrade-plan-1", "upgrade-controller-system")
	require.Len(t, annotations, 2)

	assert.Equal(t, "upgrade-plan-1", annotations["lifecycle.suse.com/upgrade-plan-name"])
	assert.Equal(t, "upgrade-controller-system", annotations["lifecycle.suse.com/upgrade-plan-namespace"])
}

func TestBaseUpgradePlan_DrainEnabled(t *testing.T) {
	upgradePlan := baseUpgradePlan("upgrade-plan-1", false, nil)

	assert.Equal(t, "Plan", upgradePlan.TypeMeta.Kind)
	assert.Equal(t, "upgrade.cattle.io/v1", upgradePlan.TypeMeta.APIVersion)

	assert.Equal(t, "upgrade-plan-1", upgradePlan.ObjectMeta.Name)
	assert.Equal(t, "cattle-system", upgradePlan.ObjectMeta.Namespace)
	assert.Nil(t, upgradePlan.ObjectMeta.Annotations)

	assert.Equal(t, "system-upgrade-controller", upgradePlan.Spec.ServiceAccountName)
	assert.Nil(t, upgradePlan.Spec.Drain)
}

func TestBaseUpgradePlan_DrainDisabled(t *testing.T) {
	upgradePlan := baseUpgradePlan("upgrade-plan-1", true, nil)

	assert.Equal(t, "Plan", upgradePlan.TypeMeta.Kind)
	assert.Equal(t, "upgrade.cattle.io/v1", upgradePlan.TypeMeta.APIVersion)

	assert.Equal(t, "upgrade-plan-1", upgradePlan.ObjectMeta.Name)
	assert.Equal(t, "cattle-system", upgradePlan.ObjectMeta.Namespace)
	assert.Nil(t, upgradePlan.ObjectMeta.Annotations)

	assert.Equal(t, "system-upgrade-controller", upgradePlan.Spec.ServiceAccountName)
	require.NotNil(t, upgradePlan.Spec.Drain)
	assert.True(t, upgradePlan.Spec.Drain.Force)
	assert.Equal(t, ptr.To(true), upgradePlan.Spec.Drain.DeleteEmptydirData)
	assert.Equal(t, ptr.To(true), upgradePlan.Spec.Drain.IgnoreDaemonSets)
	assert.Equal(t, ptr.To(15*time.Minute), upgradePlan.Spec.Drain.Timeout)
}
