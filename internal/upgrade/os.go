package upgrade

import (
	"bytes"
	_ "embed"
	"fmt"
	"text/template"

	"github.com/suse-edge/upgrade-controller/pkg/release"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

//go:embed templates/os-upgrade.sh.tpl
var osUpgradeScript string

func OSUpgradeSecret(releaseOS *release.OperatingSystem) (*corev1.Secret, error) {
	const (
		scriptName = "os-upgrade.sh"
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
		ObjectMeta: v1.ObjectMeta{
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
