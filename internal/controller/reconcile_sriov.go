package controller

import (
	"context"

	lifecyclev1alpha1 "github.com/suse-edge/upgrade-controller/api/v1alpha1"
	"github.com/suse-edge/upgrade-controller/internal/upgrade"
	"github.com/suse-edge/upgrade-controller/pkg/release"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *UpgradePlanReconciler) reconcileSRIOV(ctx context.Context, upgradePlan *lifecyclev1alpha1.UpgradePlan, sriov *release.SRIOV) (ctrl.Result, error) {
	state, err := r.upgradeHelmChart(ctx, upgradePlan, &sriov.CRD)
	if err != nil {
		return ctrl.Result{}, err
	}

	conditionType := lifecyclev1alpha1.SRIOVUpgradedCondition

	if state != upgrade.ChartStateSucceeded && state != upgrade.ChartStateVersionAlreadyInstalled {
		setCondition, requeue := evaluateHelmChartState(state)
		setCondition(upgradePlan, conditionType, state.FormattedMessage(sriov.CRD.ReleaseName))

		return ctrl.Result{Requeue: requeue}, err
	}

	state, err = r.upgradeHelmChart(ctx, upgradePlan, &sriov.NetworkOperator)
	if err != nil {
		return ctrl.Result{}, err
	}

	if state == upgrade.ChartStateNotInstalled {
		setFailedCondition(upgradePlan, conditionType, dependentHelmChartMissingMessage(sriov.CRD.ReleaseName, sriov.NetworkOperator.ReleaseName))
		return ctrl.Result{Requeue: true}, nil
	}

	setCondition, requeue := evaluateHelmChartState(state)
	setCondition(upgradePlan, conditionType, state.FormattedMessage(sriov.NetworkOperator.ReleaseName))

	return ctrl.Result{Requeue: requeue}, nil
}
