package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"

	lifecyclev1alpha1 "github.com/suse-edge/upgrade-controller/api/v1alpha1"
	"github.com/suse-edge/upgrade-controller/internal/upgrade"

	"gopkg.in/yaml.v3"

	helmcattlev1 "github.com/k3s-io/helm-controller/pkg/apis/helm.cattle.io/v1"
	helmrelease "helm.sh/helm/v3/pkg/release"
	helmutil "helm.sh/helm/v3/pkg/releaseutil"
	helmstorage "helm.sh/helm/v3/pkg/storage"
	helmdriver "helm.sh/helm/v3/pkg/storage/driver"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
func (r *UpgradePlanReconciler) updateHelmChart(ctx context.Context, upgradePlan *lifecyclev1alpha1.UpgradePlan, chart *helmcattlev1.HelmChart, releaseChart *lifecyclev1alpha1.HelmChart) error {
	backoffLimit := int32(6)

	var userValues *apiextensionsv1.JSON
	for _, h := range upgradePlan.Spec.Helm {
		if releaseChart.Name == h.Chart {
			userValues = h.Values
			break
		}
	}

	values, err := mergeHelmValues(chart.Spec.ValuesContent, releaseChart.Values, userValues)
	if err != nil {
		return fmt.Errorf("merging chart values: %w", err)
	}

	if chart.Annotations == nil {
		chart.Annotations = map[string]string{}
	}
	chart.Annotations[upgrade.PlanNameAnnotation] = upgradePlan.Name
	chart.Annotations[upgrade.PlanNamespaceAnnotation] = upgradePlan.Namespace
	chart.Annotations[upgrade.ReleaseAnnotation] = upgradePlan.Spec.ReleaseVersion
	chart.Spec.ChartContent = ""
	chart.Spec.Chart = releaseChart.Name
	chart.Spec.Version = releaseChart.Version
	chart.Spec.Repo = releaseChart.Repository
	chart.Spec.ValuesContent = string(values)
	chart.Spec.BackOffLimit = &backoffLimit

	return r.Update(ctx, chart)
}

// Creates a HelmChart resource in order to trigger an upgrade
// using the information from an existing Helm release.
func (r *UpgradePlanReconciler) createHelmChart(ctx context.Context, upgradePlan *lifecyclev1alpha1.UpgradePlan, installedChart *helmrelease.Release, releaseChart *lifecyclev1alpha1.HelmChart) error {
	backoffLimit := int32(6)

	var userValues *apiextensionsv1.JSON
	for _, h := range upgradePlan.Spec.Helm {
		if releaseChart.Name == h.Chart {
			userValues = h.Values
			break
		}
	}

	values, err := mergeHelmValues(installedChart.Config, releaseChart.Values, userValues)
	if err != nil {
		return fmt.Errorf("merging chart values: %w", err)
	}

	annotations := upgrade.PlanIdentifierAnnotations(upgradePlan.Name, upgradePlan.Namespace)
	annotations[upgrade.ReleaseAnnotation] = upgradePlan.Spec.ReleaseVersion

	chart := &helmcattlev1.HelmChart{
		TypeMeta: metav1.TypeMeta{
			Kind:       "HelmChart",
			APIVersion: "helm.cattle.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        installedChart.Name,
			Namespace:   upgrade.HelmChartNamespace,
			Annotations: annotations,
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

	return r.createObject(ctx, upgradePlan, chart)
}

func mergeHelmValues(installedValues any, releaseValues, userValues *apiextensionsv1.JSON) ([]byte, error) {
	values := map[string]any{}

	switch installed := installedValues.(type) {
	case string:
		if installed != "" {
			if err := yaml.Unmarshal([]byte(installed), &values); err != nil {
				return nil, fmt.Errorf("unmarshaling installed chart values: %w", err)
			}
		}
	case map[string]interface{}:
		if len(installed) != 0 {
			maps.Copy(values, installed)
		}
	default:
		return nil, fmt.Errorf("unexpected type %T of installed values", installedValues)
	}

	if releaseValues != nil && len(releaseValues.Raw) > 0 {
		var v map[string]any

		if err := json.Unmarshal(releaseValues.Raw, &v); err != nil {
			return nil, fmt.Errorf("unmarshaling additional release values: %w", err)
		}

		values = mergeMaps(values, v)
	}

	if userValues != nil && len(userValues.Raw) > 0 {
		var v map[string]any

		if err := json.Unmarshal(userValues.Raw, &v); err != nil {
			return nil, fmt.Errorf("unmarshaling additional user values: %w", err)
		}

		values = mergeMaps(values, v)
	}

	if len(values) == 0 {
		return nil, nil
	}

	v, err := yaml.Marshal(values)
	if err != nil {
		return nil, fmt.Errorf("marshaling chart values: %w", err)
	}

	return v, nil
}

func mergeMaps(m1, m2 map[string]any) map[string]any {
	out := make(map[string]any, len(m1))
	for k, v := range m1 {
		out[k] = v
	}

	for k, v := range m2 {
		if inner, ok := v.(map[string]any); ok {
			if outInner, ok := out[k].(map[string]any); ok {
				out[k] = mergeMaps(outInner, inner)
				continue
			}
		}
		out[k] = v
	}

	return out
}

func (r *UpgradePlanReconciler) upgradeHelmChart(
	ctx context.Context,
	upgradePlan *lifecyclev1alpha1.UpgradePlan,
	releaseChart *lifecyclev1alpha1.HelmChart,
	chartResource *helmcattlev1.HelmChart,
) (upgrade.HelmChartState, error) {
	helmRelease, err := retrieveHelmRelease(releaseChart.ReleaseName)
	if err != nil {
		if errors.Is(err, helmdriver.ErrReleaseNotFound) {
			return upgrade.ChartStateNotInstalled, nil
		}
		return upgrade.ChartStateUnknown, fmt.Errorf("retrieving helm release: %w", err)
	}

	if chartResource == nil {
		if helmRelease.Chart.Metadata.Version == releaseChart.Version {
			return upgrade.ChartStateVersionAlreadyInstalled, nil
		}

		return upgrade.ChartStateInProgress, r.createHelmChart(ctx, upgradePlan, helmRelease, releaseChart)
	}

	if chartResource.Spec.Version != releaseChart.Version {
		return upgrade.ChartStateInProgress, r.updateHelmChart(ctx, upgradePlan, chartResource, releaseChart)
	}

	releaseVersion := chartResource.Annotations[upgrade.ReleaseAnnotation]
	if releaseVersion != upgradePlan.Spec.ReleaseVersion {
		return upgrade.ChartStateVersionAlreadyInstalled, nil
	}

	job := &batchv1.Job{}
	if err = r.Get(ctx, types.NamespacedName{Name: chartResource.Status.JobName, Namespace: upgrade.HelmChartNamespace}, job); err != nil {
		return upgrade.ChartStateUnknown, client.IgnoreNotFound(err)
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
