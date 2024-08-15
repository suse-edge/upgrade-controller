package upgrade

import (
	"time"

	upgradecattlev1 "github.com/rancher/system-upgrade-controller/pkg/apis/upgrade.cattle.io/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	PlanNameAnnotation      = "lifecycle.suse.com/upgrade-plan-name"
	PlanNamespaceAnnotation = "lifecycle.suse.com/upgrade-plan-namespace"
	ReleaseAnnotation       = "lifecycle.suse.com/release"

	ControlPlaneLabel = "node-role.kubernetes.io/control-plane"

	HelmChartNamespace = "kube-system"
	SUCNamespace       = "cattle-system"

	controlPlaneKey = "control-plane"
	workersKey      = "workers"
)

func baseUpgradePlan(name string, drain bool) *upgradecattlev1.Plan {
	const (
		kind               = "Plan"
		apiVersion         = "upgrade.cattle.io/v1"
		serviceAccountName = "system-upgrade-controller"
	)

	plan := &upgradecattlev1.Plan{
		TypeMeta: metav1.TypeMeta{
			Kind:       kind,
			APIVersion: apiVersion,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: SUCNamespace,
		},
		Spec: upgradecattlev1.PlanSpec{
			ServiceAccountName: serviceAccountName,
		},
	}

	if drain {
		timeout := 15 * time.Minute
		deleteEmptyDirData := true
		ignoreDaemonSets := true
		plan.Spec.Drain = &upgradecattlev1.DrainSpec{
			Timeout:            &timeout,
			DeleteEmptydirData: &deleteEmptyDirData,
			IgnoreDaemonSets:   &ignoreDaemonSets,
			Force:              true,
		}
	}

	return plan
}
