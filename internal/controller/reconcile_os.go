package controller

import (
	"context"
	"fmt"

	"github.com/suse-edge/upgrade-controller/internal/upgrade"
	"github.com/suse-edge/upgrade-controller/pkg/release"
	ctrl "sigs.k8s.io/controller-runtime"
)

//lint:ignore U1000 - Temporary ignore "unused" linter error. Will be removed when function is ready to be used.
func (r *UpgradePlanReconciler) reconcileOS(ctx context.Context, releaseOS *release.OperatingSystem) (ctrl.Result, error) {
	secret, err := upgrade.OSUpgradeSecret(releaseOS)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("generating OS upgrade secret: %w", err)
	}

	if err = r.Create(ctx, secret); err != nil {
		return ctrl.Result{}, fmt.Errorf("creating OS upgrade secret: %w", err)
	}

	// TODO: OS upgrade logic

	return ctrl.Result{Requeue: true}, nil
}
