package upgrade

import (
	"bytes"
	_ "embed"
	"fmt"
	"path/filepath"
	"strings"
	"text/template"

	upgradecattlev1 "github.com/rancher/system-upgrade-controller/pkg/apis/upgrade.cattle.io/v1"
	lifecyclev1alpha1 "github.com/suse-edge/upgrade-controller/api/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	scriptName = "os-upgrade.sh"
)

//go:embed templates/os-upgrade.sh.tpl
var osUpgradeScript string

func OSUpgradeSecret(releaseOS *lifecyclev1alpha1.OperatingSystem) (*corev1.Secret, error) {
	const (
		secretName = "os-upgrade-secret"
	)

	tmpl, err := template.New(scriptName).Parse(osUpgradeScript)
	if err != nil {
		return nil, fmt.Errorf("parsing contents: %w", err)
	}

	values := struct {
		CPEScheme      string
		RepoGPGKey     string
		ZypperID       string
		Version        string
		SupportedArchs []string
	}{
		CPEScheme:      releaseOS.CPEScheme,
		RepoGPGKey:     releaseOS.RepoGPGPath,
		ZypperID:       releaseOS.ZypperID,
		Version:        releaseOS.Version,
		SupportedArchs: releaseOS.SupportedArchs,
	}

	var buff bytes.Buffer
	if err = tmpl.Execute(&buff, values); err != nil {
		return nil, fmt.Errorf("applying template: %w", err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: upgradeNamespace,
		},
		Type: corev1.SecretTypeOpaque,
		StringData: map[string]string{
			scriptName: buff.String(),
		},
	}

	return secret, nil
}

func OSControlPlanePlan(releaseVersion, secretName string, releaseOS *lifecyclev1alpha1.OperatingSystem, drain bool) *upgradecattlev1.Plan {
	controlPlanePlanName := osPlanName(controlPlaneKey, releaseOS.ZypperID, releaseOS.Version)
	controlPlanePlan := baseOSPlan(controlPlanePlanName, releaseVersion, secretName, drain)

	controlPlanePlan.Labels = map[string]string{
		"os-upgrade": "control-plane",
	}
	controlPlanePlan.Spec.Concurrency = 1
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

func OSWorkerPlan(releaseVersion, secretName string, releaseOS *lifecyclev1alpha1.OperatingSystem, drain bool) *upgradecattlev1.Plan {
	workerPlanName := osPlanName(workersKey, releaseOS.ZypperID, releaseOS.Version)
	workerPlan := baseOSPlan(workerPlanName, releaseVersion, secretName, drain)

	workerPlan.Labels = map[string]string{
		"os-upgrade": "worker",
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

	return workerPlan
}

func baseOSPlan(planName, releaseVersion, secretName string, drain bool) *upgradecattlev1.Plan {
	const (
		planImage = "registry.suse.com/bci/bci-base:15.5"
	)

	baseOSplan := baseUpgradePlan(planName, drain)

	secretPathRelativeToHost := fmt.Sprintf("/run/system-upgrade/secrets/%s", secretName)
	mountPath := filepath.Join("/host", secretPathRelativeToHost)
	baseOSplan.Spec.Secrets = []upgradecattlev1.SecretSpec{
		{
			Name: secretName,
			Path: mountPath,
		},
	}
	baseOSplan.Spec.Cordon = true
	baseOSplan.Spec.Version = releaseVersion

	baseOSplan.Spec.JobActiveDeadlineSecs = 3600

	baseOSplan.Spec.Upgrade = &upgradecattlev1.ContainerSpec{
		Image:   planImage,
		Command: []string{"chroot", "/host"},
		Args:    []string{"sh", filepath.Join(secretPathRelativeToHost, scriptName)},
	}
	return baseOSplan
}

func osPlanName(typeKey, osName, osVersion string) string {
	return fmt.Sprintf("%s-%s-%s", typeKey, strings.ToLower(osName), strings.ReplaceAll(osVersion, ".", "-"))
}
