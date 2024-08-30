package upgrade

import (
	"fmt"
	"strings"

	upgradecattlev1 "github.com/rancher/system-upgrade-controller/pkg/apis/upgrade.cattle.io/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	rke2UpgradeImage = "rancher/rke2-upgrade"
	k3sUpgradeImage  = "rancher/k3s-upgrade"
)

func kubernetesPlanName(typeKey, version, suffix string) string {
	versionReplacer := strings.NewReplacer(".", "-", "+", "-")
	return fmt.Sprintf("%s-%s-%s", typeKey, versionReplacer.Replace(version), suffix)
}

func kubernetesUpgradeImage(version string) string {
	if strings.Contains(version, "k3s") {
		return k3sUpgradeImage
	}

	return rke2UpgradeImage
}

func KubernetesControlPlanePlan(nameSuffix, version string, drain bool, annotations map[string]string) *upgradecattlev1.Plan {
	controlPlanePlanName := kubernetesPlanName(controlPlaneKey, version, nameSuffix)
	upgradeImage := kubernetesUpgradeImage(version)

	controlPlanePlan := baseUpgradePlan(controlPlanePlanName, drain, annotations)
	controlPlanePlan.Labels = map[string]string{
		"k8s-upgrade": "control-plane",
	}
	controlPlanePlan.Spec.NodeSelector = &metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{
				Key:      ControlPlaneLabel,
				Operator: "In",
				Values: []string{
					"true",
				},
			},
		},
	}
	controlPlanePlan.Spec.Concurrency = 1
	controlPlanePlan.Spec.Upgrade = &upgradecattlev1.ContainerSpec{
		Image: upgradeImage,
	}
	controlPlanePlan.Spec.Version = version
	controlPlanePlan.Spec.Cordon = true
	controlPlanePlan.Spec.Tolerations = []corev1.Toleration{
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

	return controlPlanePlan
}

func KubernetesWorkerPlan(nameSuffix, version string, drain bool, annotations map[string]string) *upgradecattlev1.Plan {
	controlPlanePlanName := kubernetesPlanName(controlPlaneKey, version, nameSuffix)
	workerPlanName := kubernetesPlanName(workersKey, version, nameSuffix)
	upgradeImage := kubernetesUpgradeImage(version)

	workerPlan := baseUpgradePlan(workerPlanName, drain, annotations)
	workerPlan.Labels = map[string]string{
		"k8s-upgrade": "worker",
	}
	workerPlan.Spec.Concurrency = 2
	workerPlan.Spec.NodeSelector = &metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{
				Key:      ControlPlaneLabel,
				Operator: "NotIn",
				Values: []string{
					"true",
				},
			},
		},
	}
	workerPlan.Spec.Prepare = &upgradecattlev1.ContainerSpec{
		Args: []string{
			"prepare",
			controlPlanePlanName,
		},
		Image: upgradeImage,
	}
	workerPlan.Spec.Upgrade = &upgradecattlev1.ContainerSpec{
		Image: upgradeImage,
	}
	workerPlan.Spec.Version = version
	workerPlan.Spec.Cordon = true

	return workerPlan
}
