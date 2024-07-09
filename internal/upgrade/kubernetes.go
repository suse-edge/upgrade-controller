package upgrade

import (
	"fmt"
	"strings"

	upgradecattlev1 "github.com/rancher/system-upgrade-controller/pkg/apis/upgrade.cattle.io/v1"
	"github.com/suse-edge/upgrade-controller/internal/plan"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	upgradeNamespace = "cattle-system"

	controlPlaneLabel = "node-role.kubernetes.io/control-plane"

	rke2UpgradeImage = "rancher/rke2-upgrade"

	ControlPlaneKey = "control-plane"
	WorkersKey      = "workers"
)

func KubernetesPlanKey(typeKey, kubernetesVersion string) types.NamespacedName {
	return types.NamespacedName{
		Name:      kubernetesPlanName(typeKey, kubernetesVersion),
		Namespace: upgradeNamespace,
	}
}

func KubernetesControlPlanePlan(version string) *upgradecattlev1.Plan {
	controlPlanePlanName := kubernetesPlanName(ControlPlaneKey, version)
	opts := kubernetesControlPlaneOpts(version)

	controlPlanePlan := plan.New(controlPlanePlanName, opts...)
	return controlPlanePlan
}

func KubernetesWorkerPlan(version string) *upgradecattlev1.Plan {
	controlPlanePlanName := kubernetesPlanName(ControlPlaneKey, version)
	workerPlanName := kubernetesPlanName(WorkersKey, version)
	opts := kubernetesWorkerOpts(version, controlPlanePlanName)

	workerPlan := plan.New(workerPlanName, opts...)
	return workerPlan
}

func kubernetesPlanName(typeKey, version string) string {
	return fmt.Sprintf("%s-%s", typeKey, strings.ReplaceAll(version, "+", "-"))
}

func kubernetesControlPlaneOpts(kubernetesVersion string) []plan.Option {
	return []plan.Option{
		plan.WithLabels(map[string]string{
			"rke2-upgrade": "control-plane",
		}),
		plan.WithNodeSelector([]metav1.LabelSelectorRequirement{
			{
				Key:      controlPlaneLabel,
				Operator: "In",
				Values: []string{
					"true",
				},
			},
		}),
		plan.WithConcurrency(1),
		plan.WithUpgradeSpec(&upgradecattlev1.ContainerSpec{
			Image: rke2UpgradeImage,
		}),
		plan.WithVersion(kubernetesVersion),
		plan.WithCordon(true),
		plan.WithTolerations([]corev1.Toleration{
			{
				Key:      "CriticalAddonsOnly",
				Operator: "Equal",
				Value:    "true",
				Effect:   "NoExecute",
			},
			{
				Key:      "node-role.kubernetes.io/control-plane",
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
		}),
	}
}

func kubernetesWorkerOpts(kubernetesVersion, controlPlanePlanName string) []plan.Option {
	return []plan.Option{
		plan.WithLabels(map[string]string{
			"rke2-upgrade": "worker",
		}),
		plan.WithConcurrency(2),
		plan.WithNodeSelector([]metav1.LabelSelectorRequirement{
			{
				Key:      controlPlaneLabel,
				Operator: "NotIn",
				Values: []string{
					"true",
				},
			},
		}),
		plan.WithPrepareSpec(&upgradecattlev1.ContainerSpec{
			Args: []string{
				"prepare",
				controlPlanePlanName,
			},
			Image: rke2UpgradeImage,
		}),
		plan.WithUpgradeSpec(&upgradecattlev1.ContainerSpec{
			Image: rke2UpgradeImage,
		}),
		plan.WithVersion(kubernetesVersion),
		plan.WithCordon(true),
		plan.WithDrain(&upgradecattlev1.DrainSpec{
			Force: true,
		}),
	}
}
