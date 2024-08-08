package controller

import (
	"context"
	"fmt"

	lifecyclev1alpha1 "github.com/suse-edge/upgrade-controller/api/v1alpha1"
	"github.com/suse-edge/upgrade-controller/internal/upgrade"
	"github.com/suse-edge/upgrade-controller/pkg/release"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *UpgradePlanReconciler) reconcileAkri(ctx context.Context, upgradePlan *lifecyclev1alpha1.UpgradePlan, akri *release.Akri) (ctrl.Result, error) {
	akriState, err := r.upgradeHelmChart(ctx, upgradePlan, &akri.Akri)
	if err != nil {
		return ctrl.Result{}, err
	}

	conditionType := lifecyclev1alpha1.AkriUpgradedCondition
	if akriState != upgrade.ChartStateSucceeded && akriState != upgrade.ChartStateVersionAlreadyInstalled {
		setCondition, requeue := evaluateHelmChartState(akriState)
		setCondition(upgradePlan, conditionType, akriState.FormattedMessage(akri.Akri.ReleaseName))

		return ctrl.Result{Requeue: requeue}, err
	}

	dashboardState, err := r.upgradeHelmChart(ctx, upgradePlan, &akri.DashboardExtension)
	if err != nil {
		return ctrl.Result{}, err
	}

	switch dashboardState {
	case upgrade.ChartStateFailed:
		msg := fmt.Sprintf("Main component '%s' upgraded successfully, but add-on component '%s' failed to upgrade", akri.Akri.ReleaseName, akri.DashboardExtension.ReleaseName)
		r.recordPlanEvent(upgradePlan, corev1.EventTypeWarning, conditionType, msg)

		fallthrough
	case upgrade.ChartStateNotInstalled, upgrade.ChartStateVersionAlreadyInstalled:
		setCondition, requeue := evaluateHelmChartState(akriState)
		setCondition(upgradePlan, conditionType, akriState.FormattedMessage(akri.Akri.ReleaseName))
		return ctrl.Result{Requeue: requeue}, nil
	default:
		setCondition, requeue := evaluateHelmChartState(dashboardState)
		setCondition(upgradePlan, conditionType, dashboardState.FormattedMessage(akri.DashboardExtension.ReleaseName))
		return ctrl.Result{Requeue: requeue}, nil
	}
}
