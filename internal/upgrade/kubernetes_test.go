package upgrade

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
)

func TestKubernetesControlPlanePlan_RKE2(t *testing.T) {
	nameSuffix := "abcdef"
	version := "v1.30.2+rke2r1"
	annotations := map[string]string{
		"lifecycle.suse.com/x": "z",
	}

	upgradePlan := KubernetesControlPlanePlan(nameSuffix, version, false, annotations)
	require.NotNil(t, upgradePlan)

	assert.Equal(t, "Plan", upgradePlan.TypeMeta.Kind)
	assert.Equal(t, "upgrade.cattle.io/v1", upgradePlan.TypeMeta.APIVersion)

	assert.Equal(t, "control-plane-v1-30-2-rke2r1-abcdef", upgradePlan.ObjectMeta.Name)
	assert.Equal(t, "cattle-system", upgradePlan.ObjectMeta.Namespace)
	assert.Equal(t, annotations, upgradePlan.ObjectMeta.Annotations)
	require.Len(t, upgradePlan.ObjectMeta.Labels, 1)
	assert.Equal(t, "control-plane", upgradePlan.ObjectMeta.Labels["k8s-upgrade"])

	require.Len(t, upgradePlan.Spec.NodeSelector.MatchLabels, 0)
	require.Len(t, upgradePlan.Spec.NodeSelector.MatchExpressions, 1)

	matchExpression := upgradePlan.Spec.NodeSelector.MatchExpressions[0]
	assert.Equal(t, "node-role.kubernetes.io/control-plane", matchExpression.Key)
	assert.EqualValues(t, "In", matchExpression.Operator)
	assert.Equal(t, []string{"true"}, matchExpression.Values)

	require.Nil(t, upgradePlan.Spec.Prepare)

	require.NotNil(t, upgradePlan.Spec.Upgrade)
	assert.Equal(t, "rancher/rke2-upgrade", upgradePlan.Spec.Upgrade.Image)
	assert.Nil(t, upgradePlan.Spec.Upgrade.Args)

	assert.Equal(t, version, upgradePlan.Spec.Version)
	assert.EqualValues(t, 1, upgradePlan.Spec.Concurrency)
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

func TestKubernetesControlPlanePlan_K3s(t *testing.T) {
	nameSuffix := "abcdef"
	version := "v1.30.2+k3s1"
	annotations := map[string]string{
		"lifecycle.suse.com/x": "z",
	}

	upgradePlan := KubernetesControlPlanePlan(nameSuffix, version, false, annotations)
	require.NotNil(t, upgradePlan)

	assert.Equal(t, "Plan", upgradePlan.TypeMeta.Kind)
	assert.Equal(t, "upgrade.cattle.io/v1", upgradePlan.TypeMeta.APIVersion)

	assert.Equal(t, "control-plane-v1-30-2-k3s1-abcdef", upgradePlan.ObjectMeta.Name)
	assert.Equal(t, "cattle-system", upgradePlan.ObjectMeta.Namespace)
	assert.Equal(t, annotations, upgradePlan.ObjectMeta.Annotations)
	require.Len(t, upgradePlan.ObjectMeta.Labels, 1)
	assert.Equal(t, "control-plane", upgradePlan.ObjectMeta.Labels["k8s-upgrade"])

	require.Len(t, upgradePlan.Spec.NodeSelector.MatchLabels, 0)
	require.Len(t, upgradePlan.Spec.NodeSelector.MatchExpressions, 1)

	matchExpression := upgradePlan.Spec.NodeSelector.MatchExpressions[0]
	assert.Equal(t, "node-role.kubernetes.io/control-plane", matchExpression.Key)
	assert.EqualValues(t, "In", matchExpression.Operator)
	assert.Equal(t, []string{"true"}, matchExpression.Values)

	require.Nil(t, upgradePlan.Spec.Prepare)

	require.NotNil(t, upgradePlan.Spec.Upgrade)
	assert.Equal(t, "rancher/k3s-upgrade", upgradePlan.Spec.Upgrade.Image)
	assert.Nil(t, upgradePlan.Spec.Upgrade.Args)

	assert.Equal(t, version, upgradePlan.Spec.Version)
	assert.EqualValues(t, 1, upgradePlan.Spec.Concurrency)
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

func TestKubernetesWorkerPlan_RKE2(t *testing.T) {
	nameSuffix := "abcdef"
	version := "v1.30.2+rke2r1"
	annotations := map[string]string{
		"lifecycle.suse.com/x": "z",
	}

	upgradePlan := KubernetesWorkerPlan(nameSuffix, version, false, annotations)
	require.NotNil(t, upgradePlan)

	assert.Equal(t, "Plan", upgradePlan.TypeMeta.Kind)
	assert.Equal(t, "upgrade.cattle.io/v1", upgradePlan.TypeMeta.APIVersion)

	assert.Equal(t, "workers-v1-30-2-rke2r1-abcdef", upgradePlan.ObjectMeta.Name)
	assert.Equal(t, "cattle-system", upgradePlan.ObjectMeta.Namespace)
	assert.Equal(t, annotations, upgradePlan.ObjectMeta.Annotations)
	require.Len(t, upgradePlan.ObjectMeta.Labels, 1)
	assert.Equal(t, "worker", upgradePlan.ObjectMeta.Labels["k8s-upgrade"])

	require.Len(t, upgradePlan.Spec.NodeSelector.MatchLabels, 0)
	require.Len(t, upgradePlan.Spec.NodeSelector.MatchExpressions, 1)

	matchExpression := upgradePlan.Spec.NodeSelector.MatchExpressions[0]
	assert.Equal(t, "node-role.kubernetes.io/control-plane", matchExpression.Key)
	assert.EqualValues(t, "NotIn", matchExpression.Operator)
	assert.Equal(t, []string{"true"}, matchExpression.Values)

	require.NotNil(t, upgradePlan.Spec.Prepare)
	assert.Equal(t, "rancher/rke2-upgrade", upgradePlan.Spec.Prepare.Image)
	assert.Equal(t, []string{"prepare", "control-plane-v1-30-2-rke2r1-abcdef"}, upgradePlan.Spec.Prepare.Args)

	require.NotNil(t, upgradePlan.Spec.Upgrade)
	assert.Equal(t, "rancher/rke2-upgrade", upgradePlan.Spec.Upgrade.Image)
	assert.Nil(t, upgradePlan.Spec.Upgrade.Args)

	assert.Equal(t, version, upgradePlan.Spec.Version)
	assert.EqualValues(t, 2, upgradePlan.Spec.Concurrency)
	assert.True(t, upgradePlan.Spec.Cordon)

	assert.Equal(t, "system-upgrade-controller", upgradePlan.Spec.ServiceAccountName)
	assert.Nil(t, upgradePlan.Spec.Drain)

	assert.Len(t, upgradePlan.Spec.Tolerations, 0)
}

func TestKubernetesWorkerPlan_K3s(t *testing.T) {
	nameSuffix := "abcdef"
	version := "v1.30.2+k3s1"
	annotations := map[string]string{
		"lifecycle.suse.com/x": "z",
	}

	upgradePlan := KubernetesWorkerPlan(nameSuffix, version, false, annotations)
	require.NotNil(t, upgradePlan)

	assert.Equal(t, "Plan", upgradePlan.TypeMeta.Kind)
	assert.Equal(t, "upgrade.cattle.io/v1", upgradePlan.TypeMeta.APIVersion)

	assert.Equal(t, "workers-v1-30-2-k3s1-abcdef", upgradePlan.ObjectMeta.Name)
	assert.Equal(t, "cattle-system", upgradePlan.ObjectMeta.Namespace)
	assert.Equal(t, annotations, upgradePlan.ObjectMeta.Annotations)
	require.Len(t, upgradePlan.ObjectMeta.Labels, 1)
	assert.Equal(t, "worker", upgradePlan.ObjectMeta.Labels["k8s-upgrade"])

	require.Len(t, upgradePlan.Spec.NodeSelector.MatchLabels, 0)
	require.Len(t, upgradePlan.Spec.NodeSelector.MatchExpressions, 1)

	matchExpression := upgradePlan.Spec.NodeSelector.MatchExpressions[0]
	assert.Equal(t, "node-role.kubernetes.io/control-plane", matchExpression.Key)
	assert.EqualValues(t, "NotIn", matchExpression.Operator)
	assert.Equal(t, []string{"true"}, matchExpression.Values)

	require.NotNil(t, upgradePlan.Spec.Prepare)
	assert.Equal(t, "rancher/k3s-upgrade", upgradePlan.Spec.Prepare.Image)
	assert.Equal(t, []string{"prepare", "control-plane-v1-30-2-k3s1-abcdef"}, upgradePlan.Spec.Prepare.Args)

	require.NotNil(t, upgradePlan.Spec.Upgrade)
	assert.Equal(t, "rancher/k3s-upgrade", upgradePlan.Spec.Upgrade.Image)
	assert.Nil(t, upgradePlan.Spec.Upgrade.Args)

	assert.Equal(t, version, upgradePlan.Spec.Version)
	assert.EqualValues(t, 2, upgradePlan.Spec.Concurrency)
	assert.True(t, upgradePlan.Spec.Cordon)

	assert.Equal(t, "system-upgrade-controller", upgradePlan.Spec.ServiceAccountName)
	assert.Nil(t, upgradePlan.Spec.Drain)

	assert.Len(t, upgradePlan.Spec.Tolerations, 0)
}
