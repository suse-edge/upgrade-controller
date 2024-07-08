/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	lifecyclev1alpha1 "github.com/suse-edge/upgrade-controller/api/v1alpha1"
)

// UpgradePlanReconciler reconciles a UpgradePlan object
type UpgradePlanReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=lifecycle.suse.com,resources=upgradeplans,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=lifecycle.suse.com,resources=upgradeplans/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=lifecycle.suse.com,resources=upgradeplans/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *UpgradePlanReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var upgradePlan lifecyclev1alpha1.UpgradePlan

	if err := r.Get(ctx, req.NamespacedName, &upgradePlan); err != nil {
		logger.Error(err, "unable to fetch UpgradePlan")

		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	logger.Info("Upgrade to the SUSE platform requested",
		"version", upgradePlan.Spec.ReleaseVersion)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *UpgradePlanReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&lifecyclev1alpha1.UpgradePlan{}).
		Complete(r)
}
