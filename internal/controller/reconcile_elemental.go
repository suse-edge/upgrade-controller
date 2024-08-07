package controller

import (
	"context"

	lifecyclev1alpha1 "github.com/suse-edge/upgrade-controller/api/v1alpha1"
	"github.com/suse-edge/upgrade-controller/internal/upgrade"
	"github.com/suse-edge/upgrade-controller/pkg/release"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *UpgradePlanReconciler) reconcileElemental(ctx context.Context, upgradePlan *lifecyclev1alpha1.UpgradePlan, elemental *release.Elemental) (ctrl.Result, error) {
	state, err := r.upgradeHelmChart(ctx, upgradePlan, &elemental.CRD)
	if err != nil {
		return ctrl.Result{}, err
	}

	conditionType := lifecyclev1alpha1.ElementalUpgradedCondition

	if state != upgrade.ChartStateSucceeded && state != upgrade.ChartStateVersionAlreadyInstalled {
		setCondition, requeue := evaluateHelmChartState(state)
		setCondition(upgradePlan, conditionType, state.FormattedMessage(elemental.CRD.ReleaseName))

		return ctrl.Result{Requeue: requeue}, err
	}

	state, err = r.upgradeHelmChart(ctx, upgradePlan, &elemental.Operator)
	if err != nil {
		return ctrl.Result{}, err
	}

	if state == upgrade.ChartStateNotInstalled {
		setFailedCondition(upgradePlan, conditionType, dependentHelmChartMissingMessage(elemental.CRD.ReleaseName, elemental.Operator.ReleaseName))
		return ctrl.Result{Requeue: true}, nil
	}

	setCondition, requeue := evaluateHelmChartState(state)
	setCondition(upgradePlan, conditionType, state.FormattedMessage(elemental.Operator.ReleaseName))

	return ctrl.Result{Requeue: requeue}, nil
}
