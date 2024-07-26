package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	lifecyclev1alpha1 "github.com/suse-edge/upgrade-controller/api/v1alpha1"
	"github.com/suse-edge/upgrade-controller/internal/upgrade"
	"github.com/suse-edge/upgrade-controller/pkg/release"

	helmcattlev1 "github.com/k3s-io/helm-controller/pkg/apis/helm.cattle.io/v1"
	helmrelease "helm.sh/helm/v3/pkg/release"
	helmutil "helm.sh/helm/v3/pkg/releaseutil"
	helmstorage "helm.sh/helm/v3/pkg/storage"
	helmdriver "helm.sh/helm/v3/pkg/storage/driver"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

func newHelmClient() (*helmstorage.Storage, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("retrieving cluster config: %w", err)
	}

	k8sClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("creating kubernetes client: %w", err)
	}

	driver := helmdriver.NewSecrets(k8sClient.CoreV1().Secrets(""))
	storage := helmstorage.Init(driver)

	return storage, nil
}

func retrieveHelmRelease(name string) (*helmrelease.Release, error) {
	helmClient, err := newHelmClient()
	if err != nil {
		return nil, fmt.Errorf("initializing helm client: %w", err)
	}

	helmReleases, err := helmClient.History(name)
	if err != nil {
		if errors.Is(err, helmdriver.ErrReleaseNotFound) {
			return nil, nil
		}
		return nil, err
	}

	helmutil.Reverse(helmReleases, helmutil.SortByRevision)
	helmRelease := helmReleases[0]

	return helmRelease, nil
}

// Updates an existing HelmChart resource in order to trigger an upgrade.
func (r *UpgradePlanReconciler) updateHelmChart(ctx context.Context, upgradePlan *lifecyclev1alpha1.UpgradePlan, chart *helmcattlev1.HelmChart, releaseChart *release.HelmChart) error {
	backoffLimit := int32(6)

	if chart.Annotations == nil {
		chart.Annotations = map[string]string{}
	}
	chart.Annotations[upgrade.PlanAnnotation] = upgradePlan.Name
	chart.Spec.ChartContent = ""
	chart.Spec.Chart = releaseChart.Name
	chart.Spec.Version = releaseChart.Version
	chart.Spec.Repo = releaseChart.Repository
	chart.Spec.BackOffLimit = &backoffLimit

	return r.Update(ctx, chart)
}

// Creates a HelmChart resource in order to trigger an upgrade
// using the information from an existing Helm release.
func (r *UpgradePlanReconciler) createHelmChart(ctx context.Context, releaseChart *release.HelmChart, installedChart *helmrelease.Release, upgradePlanName string) error {
	backoffLimit := int32(6)
	var values []byte

	if installedChart.Config != nil {
		// Use the current configuration values for the chart.
		var err error
		values, err = json.Marshal(installedChart.Config)
		if err != nil {
			return fmt.Errorf("marshaling chart values: %w", err)
		}
	}

	chart := &helmcattlev1.HelmChart{
		TypeMeta: metav1.TypeMeta{
			Kind:       "HelmChart",
			APIVersion: "helm.cattle.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      releaseChart.Name,
			Namespace: upgrade.ChartNamespace,
			Annotations: map[string]string{
				upgrade.PlanAnnotation: upgradePlanName,
			},
		},
		Spec: helmcattlev1.HelmChartSpec{
			Chart:           releaseChart.Name,
			Version:         releaseChart.Version,
			Repo:            releaseChart.Repository,
			TargetNamespace: installedChart.Namespace,
			ValuesContent:   string(values),
			BackOffLimit:    &backoffLimit,
		},
	}

	return r.Create(ctx, chart)
}
