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

func (r *UpgradePlanReconciler) reconcileKubevirt(ctx context.Context, upgradePlan *lifecyclev1alpha1.UpgradePlan, kubevirt *release.KubeVirt) (ctrl.Result, error) {
	kubevirtState, err := r.upgradeHelmChart(ctx, upgradePlan, &kubevirt.KubeVirt)
	if err != nil {
		return ctrl.Result{}, err
	}

	conditionType := lifecyclev1alpha1.KubevirtUpgradedCondition
	if kubevirtState != upgrade.ChartStateSucceeded && kubevirtState != upgrade.ChartStateVersionAlreadyInstalled {
		setCondition, requeue := evaluateHelmChartState(kubevirtState)
		setCondition(upgradePlan, conditionType, kubevirtState.FormattedMessage(kubevirt.KubeVirt.ReleaseName))

		return ctrl.Result{Requeue: requeue}, err
	}

	dashboardState, err := r.upgradeHelmChart(ctx, upgradePlan, &kubevirt.DashboardExtension)
	if err != nil {
		return ctrl.Result{}, err
	}

	switch dashboardState {
	case upgrade.ChartStateFailed:
		msg := fmt.Sprintf("Main component '%s' upgraded successfully, but add-on component '%s' failed to upgrade", kubevirt.KubeVirt.ReleaseName, kubevirt.DashboardExtension.ReleaseName)
		r.recordPlanEvent(upgradePlan, corev1.EventTypeWarning, conditionType, msg)

		fallthrough
	case upgrade.ChartStateNotInstalled, upgrade.ChartStateVersionAlreadyInstalled:
		setCondition, requeue := evaluateHelmChartState(kubevirtState)
		setCondition(upgradePlan, conditionType, kubevirtState.FormattedMessage(kubevirt.KubeVirt.ReleaseName))
		return ctrl.Result{Requeue: requeue}, nil
	default:
		setCondition, requeue := evaluateHelmChartState(dashboardState)
		setCondition(upgradePlan, conditionType, dashboardState.FormattedMessage(kubevirt.DashboardExtension.ReleaseName))
		return ctrl.Result{Requeue: requeue}, nil
	}
}
