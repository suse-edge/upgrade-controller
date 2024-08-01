package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	lifecyclev1alpha1 "github.com/suse-edge/upgrade-controller/api/v1alpha1"
	"github.com/suse-edge/upgrade-controller/internal/upgrade"
	"github.com/suse-edge/upgrade-controller/pkg/release"

	helmcattlev1 "github.com/k3s-io/helm-controller/pkg/apis/helm.cattle.io/v1"
	helmrelease "helm.sh/helm/v3/pkg/release"
	helmutil "helm.sh/helm/v3/pkg/releaseutil"
	helmstorage "helm.sh/helm/v3/pkg/storage"
	helmdriver "helm.sh/helm/v3/pkg/storage/driver"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/log"
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
		return nil, err
	}

	if len(helmReleases) == 0 {
		return nil, helmdriver.ErrReleaseNotFound
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
	chart.Annotations[upgrade.ReleaseAnnotation] = upgradePlan.Spec.ReleaseVersion
	chart.Spec.ChartContent = ""
	chart.Spec.Chart = releaseChart.Name
	chart.Spec.Version = releaseChart.Version
	chart.Spec.Repo = releaseChart.Repository
	chart.Spec.BackOffLimit = &backoffLimit

	return r.Update(ctx, chart)
}

// Creates a HelmChart resource in order to trigger an upgrade
// using the information from an existing Helm release.
func (r *UpgradePlanReconciler) createHelmChart(ctx context.Context, upgradePlan *lifecyclev1alpha1.UpgradePlan, installedChart *helmrelease.Release, releaseChart *release.HelmChart) error {
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
			Name:      installedChart.Name,
			Namespace: upgrade.ChartNamespace,
			Annotations: map[string]string{
				upgrade.PlanAnnotation:    upgradePlan.Name,
				upgrade.ReleaseAnnotation: upgradePlan.Spec.ReleaseVersion,
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

func (r *UpgradePlanReconciler) upgradeHelmChart(ctx context.Context, upgradePlan *lifecyclev1alpha1.UpgradePlan, releaseChart *release.HelmChart) (upgrade.HelmChartState, error) {
	helmReleaseName := retrieveHelmReleaseName(releaseChart)
	helmRelease, err := retrieveHelmRelease(helmReleaseName)
	if err != nil {
		if errors.Is(err, helmdriver.ErrReleaseNotFound) {
			return upgrade.ChartStateNotInstalled, nil
		}
		return upgrade.ChartStateUnknown, fmt.Errorf("retrieving helm release: %w", err)
	}

	chart := &helmcattlev1.HelmChart{}

	if err = r.Get(ctx, upgrade.ChartNamespacedName(helmRelease.Name), chart); err != nil {
		if !apierrors.IsNotFound(err) {
			return upgrade.ChartStateUnknown, err
		}

		if helmRelease.Chart.Metadata.Version == releaseChart.Version {
			return upgrade.ChartStateVersionAlreadyInstalled, nil
		}

		return upgrade.ChartStateInProgress, r.createHelmChart(ctx, upgradePlan, helmRelease, releaseChart)
	}

	if chart.Spec.Version != releaseChart.Version {
		return upgrade.ChartStateInProgress, r.updateHelmChart(ctx, upgradePlan, chart, releaseChart)
	}

	releaseVersion := chart.Annotations[upgrade.ReleaseAnnotation]
	if releaseVersion != upgradePlan.Spec.ReleaseVersion {
		return upgrade.ChartStateVersionAlreadyInstalled, nil
	}

	job := &batchv1.Job{}
	if err = r.Get(ctx, types.NamespacedName{Name: chart.Status.JobName, Namespace: upgrade.ChartNamespace}, job); err != nil {
		return upgrade.ChartStateUnknown, err
	}

	idx := slices.IndexFunc(job.Status.Conditions, func(condition batchv1.JobCondition) bool {
		return condition.Status == corev1.ConditionTrue &&
			(condition.Type == batchv1.JobComplete || condition.Type == batchv1.JobFailed)
	})

	if idx == -1 {
		// Upgrade job is still ongoing.
		return upgrade.ChartStateInProgress, nil
	}

	condition := job.Status.Conditions[idx]
	if condition.Type == batchv1.JobComplete {
		return upgrade.ChartStateSucceeded, nil
	}

	logger := log.FromContext(ctx)
	logger.Info("Helm chart upgrade job failed",
		"helmChart", releaseChart.Name,
		"job", fmt.Sprintf("%s/%s", job.Namespace, job.Name),
		"jobStatus", condition.Message)

	return upgrade.ChartStateFailed, nil
}

func retrieveHelmReleaseName(releaseChart *release.HelmChart) string {
	if releaseChart.ReleaseName != "" {
		return releaseChart.ReleaseName
	}

	return releaseChart.Name
}

func evaluateHelmChartState(state upgrade.HelmChartState) (setCondition setCondition, requeue bool) {
	switch state {
	case upgrade.ChartStateNotInstalled, upgrade.ChartStateVersionAlreadyInstalled:
		return setSkippedCondition, true
	case upgrade.ChartStateInProgress:
		return setInProgressCondition, false
	case upgrade.ChartStateSucceeded:
		return setSuccessfulCondition, true
	case upgrade.ChartStateFailed:
		return setFailedCondition, true
	default:
		return setErrorCondition, false
	}
}
