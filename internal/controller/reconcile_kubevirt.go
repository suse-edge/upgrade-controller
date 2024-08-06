package controller

import (
	"context"

	lifecyclev1alpha1 "github.com/suse-edge/upgrade-controller/api/v1alpha1"
	"github.com/suse-edge/upgrade-controller/internal/upgrade"
	"github.com/suse-edge/upgrade-controller/pkg/release"

	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *UpgradePlanReconciler) reconcileKubevirt(ctx context.Context, upgradePlan *lifecyclev1alpha1.UpgradePlan, kubevirt *release.KubeVirt) (ctrl.Result, error) {
	state, err := r.upgradeHelmChart(ctx, upgradePlan, &kubevirt.KubeVirt)
	if err != nil {
		return ctrl.Result{}, err
	}

	conditionType := lifecyclev1alpha1.KubevirtUpgradedCondition
	if state != upgrade.ChartStateSucceeded && state != upgrade.ChartStateVersionAlreadyInstalled {
		setCondition, requeue := evaluateHelmChartState(state)
		setCondition(upgradePlan, conditionType, state.FormattedMessage(kubevirt.KubeVirt.ReleaseName))

		return ctrl.Result{Requeue: requeue}, err
	}

	state, err = r.upgradeHelmChart(ctx, upgradePlan, &kubevirt.DashboardExtension)
	if err != nil {
		return ctrl.Result{}, err
	}

	setCondition, requeue := evaluateHelmChartState(state)
	setCondition(upgradePlan, conditionType, state.FormattedMessage(kubevirt.DashboardExtension.ReleaseName))

	return ctrl.Result{Requeue: requeue}, err
}
