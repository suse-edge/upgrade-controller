package upgrade

import (
	"bytes"
	_ "embed"
	"fmt"
	"path/filepath"
	"strings"
	"text/template"

	upgradecattlev1 "github.com/rancher/system-upgrade-controller/pkg/apis/upgrade.cattle.io/v1"
	"github.com/suse-edge/upgrade-controller/pkg/release"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	scriptName = "os-upgrade.sh"
	secretName = "os-upgrade-secret"
)

//go:embed templates/os-upgrade.sh.tpl
var osUpgradeScript string

func OSUpgradeSecret(releaseOS *release.OperatingSystem) (*corev1.Secret, error) {
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
			Namespace: planNamespace,
		},
		Type: corev1.SecretTypeOpaque,
		StringData: map[string]string{
			scriptName: buff.String(),
		},
	}

	return secret, nil
}

func OSControlPlanePlan(releaseVersion string, releaseOS *release.OperatingSystem) *upgradecattlev1.Plan {
	const (
		planImage = "registry.suse.com/bci/bci-base:15.5"
	)

	controlPlanePlanName := osPlanName(controlPlaneKey, releaseOS.ZypperID, releaseOS.Version)
	controlPlanePlan := baseUpgradePlan(controlPlanePlanName)
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

	secretPathRelativeToHost := fmt.Sprintf("/run/system-upgrade/secrets/%s", secretName)
	mountPath := filepath.Join("/host", secretPathRelativeToHost)
	controlPlanePlan.Spec.Secrets = []upgradecattlev1.SecretSpec{
		{
			Name: secretName,
			Path: mountPath,
		},
	}
	controlPlanePlan.Spec.Cordon = true
	controlPlanePlan.Spec.Version = releaseVersion

	controlPlanePlan.Spec.Upgrade = &upgradecattlev1.ContainerSpec{
		Image:   planImage,
		Command: []string{"chroot", "/host"},
		Args:    []string{"sh", filepath.Join(secretPathRelativeToHost, scriptName)},
	}
	return controlPlanePlan
}

func osPlanName(typeKey, osName, osVersion string) string {
	return fmt.Sprintf("%s-%s-%s", typeKey, strings.ToLower(osName), strings.ReplaceAll(osVersion, ".", "-"))
}
