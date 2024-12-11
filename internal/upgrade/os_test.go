package upgrade

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	lifecyclev1alpha1 "github.com/suse-edge/upgrade-controller/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

const (
	planNameSuffix = "abcdef"
	releaseVersion = "3.1.0"
)

func TestOSUpgradeSecret(t *testing.T) {
	os := &lifecyclev1alpha1.OperatingSystem{
		Version:   "6.0",
		ZypperID:  "SL-Micro",
		CPEScheme: "some-cpe-scheme",
	}
	labels := map[string]string{
		"lifecycle.suse.com/x": "z",
	}

	secret, err := OSUpgradeSecret(planNameSuffix, os, labels)
	require.NoError(t, err)

	assert.Equal(t, "Secret", secret.TypeMeta.Kind)
	assert.Equal(t, "v1", secret.TypeMeta.APIVersion)

	assert.Equal(t, "os-upgrade-secret-sl-micro-6-0-abcdef", secret.ObjectMeta.Name)
	assert.Equal(t, "cattle-system", secret.ObjectMeta.Namespace)
	assert.Equal(t, labels, secret.ObjectMeta.Labels)

	assert.EqualValues(t, "Opaque", secret.Type)

	require.Len(t, secret.StringData, 1)
	scriptContents := secret.StringData["os-upgrade.sh"]
	require.NotEmpty(t, scriptContents)

	assert.Contains(t, scriptContents, "RELEASE_CPE=some-cpe-scheme")
	assert.Contains(t, scriptContents, "/usr/sbin/transactional-update --continue run zypper migration --gpg-auto-import-keys --non-interactive --product SL-Micro/6.0/${SYSTEM_ARCH} --root /")
}

func TestOSControlPlanePlan(t *testing.T) {
	secretName := "some-secret"
	os := &lifecyclev1alpha1.OperatingSystem{
		Version:  "6.0",
		ZypperID: "SL-Micro",
	}
	addLabels := map[string]string{
		"lifecycle.suse.com/x": "z",
	}

	expectedLabels := map[string]string{
		"lifecycle.suse.com/x": "z",
		"os-upgrade":           "control-plane",
	}

	upgradePlan := OSControlPlanePlan(planNameSuffix, releaseVersion, secretName, os, false, addLabels)
	require.NotNil(t, upgradePlan)

	assert.Equal(t, "Plan", upgradePlan.TypeMeta.Kind)
	assert.Equal(t, "upgrade.cattle.io/v1", upgradePlan.TypeMeta.APIVersion)

	assert.Equal(t, "control-plane-sl-micro-6-0-abcdef", upgradePlan.ObjectMeta.Name)
	assert.Equal(t, "cattle-system", upgradePlan.ObjectMeta.Namespace)
	assert.Equal(t, expectedLabels, upgradePlan.ObjectMeta.Labels)

	require.Len(t, upgradePlan.Spec.NodeSelector.MatchLabels, 0)
	require.Len(t, upgradePlan.Spec.NodeSelector.MatchExpressions, 1)

	matchExpression := upgradePlan.Spec.NodeSelector.MatchExpressions[0]
	assert.Equal(t, "node-role.kubernetes.io/control-plane", matchExpression.Key)
	assert.EqualValues(t, "In", matchExpression.Operator)
	assert.Equal(t, []string{"true"}, matchExpression.Values)

	require.Nil(t, upgradePlan.Spec.Prepare)

	upgradeContainer := upgradePlan.Spec.Upgrade
	require.NotNil(t, upgradeContainer)
	assert.Equal(t, "registry.suse.com/bci/bci-base:15.6", upgradeContainer.Image)
	assert.Equal(t, []string{"chroot", "/host"}, upgradeContainer.Command)
	assert.Equal(t, []string{"sh", "/run/system-upgrade/secrets/some-secret/os-upgrade.sh"}, upgradeContainer.Args)

	assert.Equal(t, "3.1.0", upgradePlan.Spec.Version)
	assert.EqualValues(t, 1, upgradePlan.Spec.Concurrency)
	assert.EqualValues(t, 43200, upgradePlan.Spec.JobActiveDeadlineSecs)
	assert.True(t, upgradePlan.Spec.Cordon)

	assert.Equal(t, "system-upgrade-controller", upgradePlan.Spec.ServiceAccountName)
	assert.Nil(t, upgradePlan.Spec.Drain)

	tolerations := []corev1.Toleration{
		{
			Key:      "CriticalAddonsOnly",
			Operator: "Equal",
			Value:    "true",
			Effect:   "NoExecute",
		},
		{
			Key:      ControlPlaneLabel,
			Operator: "Equal",
			Value:    "",
			Effect:   "NoSchedule",
		},
		{
			Key:      "node-role.kubernetes.io/etcd",
			Operator: "Equal",
			Value:    "",
			Effect:   "NoExecute",
		},
	}
	assert.Equal(t, tolerations, upgradePlan.Spec.Tolerations)
}

func TestOSWorkerPlan(t *testing.T) {
	secretName := "some-secret"
	os := &lifecyclev1alpha1.OperatingSystem{
		Version:  "6.0",
		ZypperID: "SL-Micro",
	}
	addLabels := map[string]string{
		"lifecycle.suse.com/x": "z",
	}

	expectedLabels := map[string]string{
		"lifecycle.suse.com/x": "z",
		"os-upgrade":           "worker",
	}

	upgradePlan := OSWorkerPlan(planNameSuffix, releaseVersion, secretName, os, false, addLabels)
	require.NotNil(t, upgradePlan)

	assert.Equal(t, "Plan", upgradePlan.TypeMeta.Kind)
	assert.Equal(t, "upgrade.cattle.io/v1", upgradePlan.TypeMeta.APIVersion)

	assert.Equal(t, "workers-sl-micro-6-0-abcdef", upgradePlan.ObjectMeta.Name)
	assert.Equal(t, "cattle-system", upgradePlan.ObjectMeta.Namespace)
	assert.Equal(t, expectedLabels, upgradePlan.ObjectMeta.Labels)

	require.Len(t, upgradePlan.Spec.NodeSelector.MatchLabels, 0)
	require.Len(t, upgradePlan.Spec.NodeSelector.MatchExpressions, 1)

	matchExpression := upgradePlan.Spec.NodeSelector.MatchExpressions[0]
	assert.Equal(t, "node-role.kubernetes.io/control-plane", matchExpression.Key)
	assert.EqualValues(t, "NotIn", matchExpression.Operator)
	assert.Equal(t, []string{"true"}, matchExpression.Values)

	require.Nil(t, upgradePlan.Spec.Prepare)

	upgradeContainer := upgradePlan.Spec.Upgrade
	require.NotNil(t, upgradeContainer)
	assert.Equal(t, "registry.suse.com/bci/bci-base:15.6", upgradeContainer.Image)
	assert.Equal(t, []string{"chroot", "/host"}, upgradeContainer.Command)
	assert.Equal(t, []string{"sh", "/run/system-upgrade/secrets/some-secret/os-upgrade.sh"}, upgradeContainer.Args)

	assert.Equal(t, "3.1.0", upgradePlan.Spec.Version)
	assert.EqualValues(t, 1, upgradePlan.Spec.Concurrency)
	assert.EqualValues(t, 43200, upgradePlan.Spec.JobActiveDeadlineSecs)
	assert.True(t, upgradePlan.Spec.Cordon)

	assert.Equal(t, "system-upgrade-controller", upgradePlan.Spec.ServiceAccountName)
	assert.Nil(t, upgradePlan.Spec.Drain)

	assert.Len(t, upgradePlan.Spec.Tolerations, 0)
}
