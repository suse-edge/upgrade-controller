package controller

import (
	"context"

	lifecyclev1alpha1 "github.com/suse-edge/upgrade-controller/api/v1alpha1"
	"github.com/suse-edge/upgrade-controller/pkg/release"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *UpgradePlanReconciler) reconcileLonghorn(ctx context.Context, upgradePlan *lifecyclev1alpha1.UpgradePlan, longhorn *release.HelmChart) (ctrl.Result, error) {
	state, err := r.upgradeHelmChart(ctx, upgradePlan, longhorn)
	if err != nil {
		return ctrl.Result{}, err
	}

	setCondition, requeue := evaluateHelmChartState(state)
	if setCondition != nil {
		setCondition(upgradePlan, lifecyclev1alpha1.LonghornUpgradedCondition, state.Message())
	}

	return ctrl.Result{Requeue: requeue}, nil
}
