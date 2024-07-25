package controller

import (
	"context"
	"fmt"

	lifecyclev1alpha1 "github.com/suse-edge/upgrade-controller/api/v1alpha1"
	"github.com/suse-edge/upgrade-controller/internal/upgrade"
	"github.com/suse-edge/upgrade-controller/pkg/release"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

//lint:ignore U1000 - Temporary ignore "unused" linter error. Will be removed when function is ready to be used.
func (r *UpgradePlanReconciler) reconcileOS(ctx context.Context, upgradePlan *lifecyclev1alpha1.UpgradePlan, release *release.Release) (ctrl.Result, error) {
	secret, err := upgrade.OSUpgradeSecret(&release.Components.OperatingSystem)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("generating OS upgrade secret: %w", err)
	}

	if err = r.Get(ctx, client.ObjectKeyFromObject(secret), secret); err != nil {
		if !errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, r.createSecret(ctx, upgradePlan, secret)
	}

	controlPlanePlan := upgrade.OSControlPlanePlan(release.ReleaseVersion, &release.Components.OperatingSystem)
	if err = r.Get(ctx, client.ObjectKeyFromObject(controlPlanePlan), controlPlanePlan); err != nil {
		if !errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}

		setInProgressCondition(upgradePlan, lifecyclev1alpha1.OperatingSystemUpgradedCondition, "Control plane nodes are being upgraded")
		return ctrl.Result{}, r.createPlan(ctx, upgradePlan, controlPlanePlan)
	}

	return ctrl.Result{Requeue: true}, nil
}
