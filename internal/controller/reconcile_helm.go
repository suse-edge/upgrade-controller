package controller

import (
	"context"
	"fmt"

	lifecyclev1alpha1 "github.com/suse-edge/upgrade-controller/api/v1alpha1"
	"github.com/suse-edge/upgrade-controller/internal/upgrade"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *UpgradePlanReconciler) reconcileHelmChart(ctx context.Context, upgradePlan *lifecyclev1alpha1.UpgradePlan, chart *lifecyclev1alpha1.HelmChart) (ctrl.Result, error) {
	conditionType := lifecyclev1alpha1.GetChartConditionType(chart.PrettyName)

	if len(chart.DependencyCharts) != 0 {
		for _, depChart := range chart.DependencyCharts {
			depState, err := r.upgradeHelmChart(ctx, upgradePlan, &depChart)
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

	coreState, err := r.upgradeHelmChart(ctx, upgradePlan, chart)
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
			addonState, err := r.upgradeHelmChart(ctx, upgradePlan, &addonChart)
			if err != nil {
				return ctrl.Result{}, err
			}

			switch addonState {
			case upgrade.ChartStateFailed:
				msg := fmt.Sprintf("'%s' upgraded successfully, but add-on component '%s' failed to upgrade", chart.ReleaseName, addonChart.ReleaseName)
				r.recordPlanEvent(upgradePlan, corev1.EventTypeWarning, conditionType, msg)
			case upgrade.ChartStateNotInstalled:
				msg := fmt.Sprintf("'%s' add-on component upgrade skipped as it is missing in the cluster", addonChart.ReleaseName)
				r.recordPlanEvent(upgradePlan, corev1.EventTypeNormal, conditionType, msg)
			case upgrade.ChartStateSucceeded:
				msg := fmt.Sprintf("'%s' add-on component successfully upgraded", addonChart.ReleaseName)
				r.recordPlanEvent(upgradePlan, corev1.EventTypeNormal, conditionType, msg)
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
