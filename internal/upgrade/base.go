package upgrade

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	upgradecattlev1 "github.com/rancher/system-upgrade-controller/pkg/apis/upgrade.cattle.io/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	PlanNameLabel      = "lifecycle.suse.com/upgrade-plan-name"
	PlanNamespaceLabel = "lifecycle.suse.com/upgrade-plan-namespace"

	ReleaseAnnotation = "lifecycle.suse.com/release"

	ControlPlaneLabel = "node-role.kubernetes.io/control-plane"

	HelmChartNamespace = "kube-system"
	SUCNamespace       = "cattle-system"

	controlPlaneKey = "control-plane"
	workersKey      = "workers"

	// 5 random bytes = 10 random hexadecimal characters
	randomByteNum = 5
)

func PlanIdentifierLabels(name, namespace string) map[string]string {
	return map[string]string{
		PlanNameLabel:      name,
		PlanNamespaceLabel: namespace,
	}
}

func GenerateSuffix() (string, error) {
	bytes := make([]byte, randomByteNum)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generating random bytes: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

func baseUpgradePlan(name string, drain bool, labels map[string]string) *upgradecattlev1.Plan {
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
			Labels:    labels,
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
