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

	controlPlaneKey = "control-plane"
	workersKey      = "workers"
)

func kubernetesPlanName(typeKey, version string) string {
	return fmt.Sprintf("%s-%s", typeKey, strings.ReplaceAll(version, "+", "-"))
}

func KubernetesControlPlanePlan(version string) *upgradecattlev1.Plan {
	controlPlanePlanName := kubernetesPlanName(controlPlaneKey, version)

	controlPlanePlan := baseUpgradePlan(controlPlanePlanName)
	controlPlanePlan.Labels = map[string]string{
		"rke2-upgrade": "control-plane",
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
		Image: rke2UpgradeImage,
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

func KubernetesWorkerPlan(version string) *upgradecattlev1.Plan {
	controlPlanePlanName := kubernetesPlanName(controlPlaneKey, version)
	workerPlanName := kubernetesPlanName(workersKey, version)

	workerPlan := baseUpgradePlan(workerPlanName)
	workerPlan.Labels = map[string]string{
		"rke2-upgrade": "worker",
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
		Image: rke2UpgradeImage,
	}
	workerPlan.Spec.Upgrade = &upgradecattlev1.ContainerSpec{
		Image: rke2UpgradeImage,
	}
	workerPlan.Spec.Version = version
	workerPlan.Spec.Cordon = true
	workerPlan.Spec.Drain = &upgradecattlev1.DrainSpec{
		Force: true,
	}

	return workerPlan
}
