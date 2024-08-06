package controller

import (
	"context"

	lifecyclev1alpha1 "github.com/suse-edge/upgrade-controller/api/v1alpha1"
	"github.com/suse-edge/upgrade-controller/pkg/release"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *UpgradePlanReconciler) reconcileRancher(ctx context.Context, upgradePlan *lifecyclev1alpha1.UpgradePlan, rancher *release.HelmChart) (ctrl.Result, error) {
	state, err := r.upgradeHelmChart(ctx, upgradePlan, rancher)
	if err != nil {
		return ctrl.Result{}, err
	}

	setCondition, requeue := evaluateHelmChartState(state)
	setCondition(upgradePlan, lifecyclev1alpha1.RancherUpgradedCondition, state.FormattedMessage(rancher.ReleaseName))

	return ctrl.Result{Requeue: requeue}, nil
}
