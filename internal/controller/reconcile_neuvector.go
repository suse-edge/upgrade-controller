package controller

import (
	"context"

	lifecyclev1alpha1 "github.com/suse-edge/upgrade-controller/api/v1alpha1"
	"github.com/suse-edge/upgrade-controller/internal/upgrade"
	"github.com/suse-edge/upgrade-controller/pkg/release"
	ctrl "sigs.k8s.io/controller-runtime"
)

func (r *UpgradePlanReconciler) reconcileNeuVector(ctx context.Context, upgradePlan *lifecyclev1alpha1.UpgradePlan, neuVector *release.NeuVector) (ctrl.Result, error) {
	state, err := r.upgradeHelmChart(ctx, upgradePlan, &neuVector.CRD)
	if err != nil {
		return ctrl.Result{}, err
	}

	conditionType := lifecyclev1alpha1.NeuVectorUpgradedCondition

	if state != upgrade.ChartStateSucceeded && state != upgrade.ChartStateVersionAlreadyInstalled {
		setCondition, requeue := evaluateHelmChartState(state)
		setCondition(upgradePlan, conditionType, state.FormattedMessage(neuVector.CRD.ReleaseName))

		return ctrl.Result{Requeue: requeue}, err
	}

	state, err = r.upgradeHelmChart(ctx, upgradePlan, &neuVector.NeuVector)
	if err != nil {
		return ctrl.Result{}, err
	}

	if state == upgrade.ChartStateNotInstalled {
		setFailedCondition(upgradePlan, conditionType, dependentHelmChartMissingMessage(neuVector.CRD.ReleaseName, neuVector.NeuVector.ReleaseName))
		return ctrl.Result{Requeue: true}, nil
	}

	setCondition, requeue := evaluateHelmChartState(state)
	setCondition(upgradePlan, conditionType, state.FormattedMessage(neuVector.NeuVector.ReleaseName))

	return ctrl.Result{Requeue: requeue}, err
}
