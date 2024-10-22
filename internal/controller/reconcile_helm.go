package controller

import (
	"context"
	"fmt"

	helmcattlev1 "github.com/k3s-io/helm-controller/pkg/apis/helm.cattle.io/v1"
	lifecyclev1alpha1 "github.com/suse-edge/upgrade-controller/api/v1alpha1"
	"github.com/suse-edge/upgrade-controller/internal/upgrade"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *UpgradePlanReconciler) reconcileHelmChart(ctx context.Context, upgradePlan *lifecyclev1alpha1.UpgradePlan, chart *lifecyclev1alpha1.HelmChart) (ctrl.Result, error) {
	chartResources := &helmcattlev1.HelmChartList{}

	if err := r.List(ctx, chartResources); err != nil {
		return ctrl.Result{}, fmt.Errorf("listing HelmChart resources: %w", err)
	}

	conditionType := lifecyclev1alpha1.GetChartConditionType(chart.PrettyName)

	if len(chart.DependencyCharts) != 0 {
		for _, depChart := range chart.DependencyCharts {
			chartResource, err := findChartResource(chartResources, depChart.ReleaseName)
			if err != nil {
				return ctrl.Result{}, err
			}

			depState, err := r.upgradeHelmChart(ctx, upgradePlan, &depChart, chartResource)
			if err != nil {
				return ctrl.Result{}, err
			}

			if depState != upgrade.ChartStateSucceeded && depState != upgrade.ChartStateVersionAlreadyInstalled {
				setCondition, requeue := evaluateHelmChartState(depState)
				setCondition(upgradePlan, conditionType, depState.FormattedMessage(depChart.ReleaseName))

				return ctrl.Result{Requeue: requeue}, nil
			}
		}
	}

	chartResource, err := findChartResource(chartResources, chart.ReleaseName)
	if err != nil {
		return ctrl.Result{}, err
	}

	coreState, err := r.upgradeHelmChart(ctx, upgradePlan, chart, chartResource)
	if err != nil {
		return ctrl.Result{}, err
	}

	if coreState == upgrade.ChartStateNotInstalled && len(chart.DependencyCharts) != 0 {
		setFailedCondition(upgradePlan, conditionType, fmt.Sprintf("'%s' core chart is missing, but dependency charts are present", chart.ReleaseName))
		return ctrl.Result{Requeue: true}, nil
	}

	if coreState != upgrade.ChartStateSucceeded && coreState != upgrade.ChartStateVersionAlreadyInstalled {
		setCondition, requeue := evaluateHelmChartState(coreState)
		setCondition(upgradePlan, conditionType, coreState.FormattedMessage(chart.ReleaseName))

		return ctrl.Result{Requeue: requeue}, nil
	}

	if len(chart.AddonCharts) != 0 {
		for _, addonChart := range chart.AddonCharts {
			chartResource, err = findChartResource(chartResources, addonChart.ReleaseName)
			if err != nil {
				return ctrl.Result{}, err
			}

			addonState, err := r.upgradeHelmChart(ctx, upgradePlan, &addonChart, chartResource)
			if err != nil {
				return ctrl.Result{}, err
			}

			switch addonState {
			case upgrade.ChartStateFailed:
				r.Recorder.Eventf(upgradePlan, corev1.EventTypeWarning, conditionType,
					"'%s' upgraded successfully, but add-on component '%s' failed to upgrade", chart.ReleaseName, addonChart.ReleaseName)
			case upgrade.ChartStateNotInstalled:
				r.Recorder.Eventf(upgradePlan, corev1.EventTypeNormal, conditionType,
					"'%s' add-on component upgrade skipped as it is missing in the cluster", addonChart.ReleaseName)
			case upgrade.ChartStateSucceeded:
				r.Recorder.Eventf(upgradePlan, corev1.EventTypeNormal, conditionType,
					"'%s' add-on component successfully upgraded", addonChart.ReleaseName)
			case upgrade.ChartStateInProgress:
				// mark that current add-on chart upgrade is in progress
				setInProgressCondition(upgradePlan, conditionType, addonState.FormattedMessage(addonChart.ReleaseName))
				return ctrl.Result{Requeue: true}, nil
			case upgrade.ChartStateUnknown:
				return ctrl.Result{}, nil
			}
		}
	}

	// to avoid confusion, when upgrade has been done, use core component message in the component condition
	setCondition, requeue := evaluateHelmChartState(coreState)
	setCondition(upgradePlan, conditionType, coreState.FormattedMessage(chart.ReleaseName))
	return ctrl.Result{Requeue: requeue}, nil
}

func findChartResource(helmCharts *helmcattlev1.HelmChartList, name string) (*helmcattlev1.HelmChart, error) {
	var charts []helmcattlev1.HelmChart

	for _, chart := range helmCharts.Items {
		if chart.Name == name {
			charts = append(charts, chart)
		}
	}

	switch len(charts) {
	case 0:
		return nil, nil
	case 1:
		return &charts[0], nil
	default:
		return nil, fmt.Errorf("more than one HelmChart resource with name '%s' exists", name)
	}
}
