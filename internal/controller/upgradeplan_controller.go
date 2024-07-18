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
	"errors"
	"fmt"

	upgradecattlev1 "github.com/rancher/system-upgrade-controller/pkg/apis/upgrade.cattle.io/v1"
	lifecyclev1alpha1 "github.com/suse-edge/upgrade-controller/api/v1alpha1"
	"github.com/suse-edge/upgrade-controller/internal/upgrade"
	"github.com/suse-edge/upgrade-controller/pkg/release"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// UpgradePlanReconciler reconciles a UpgradePlan object
type UpgradePlanReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder

	Releases map[string]*release.Release
}

// +kubebuilder:rbac:groups=lifecycle.suse.com,resources=upgradeplans,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=lifecycle.suse.com,resources=upgradeplans/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=lifecycle.suse.com,resources=upgradeplans/finalizers,verbs=update
// +kubebuilder:rbac:groups=upgrade.cattle.io,resources=plans,verbs=create;list;get;watch
// +kubebuilder:rbac:groups="",resources=nodes,verbs=watch;list
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *UpgradePlanReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	plan := &lifecyclev1alpha1.UpgradePlan{}

	if err := r.Get(ctx, req.NamespacedName, plan); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	logger := log.FromContext(ctx)
	logger.Info("Reconciling UpgradePlan")

	result, err := r.executePlan(ctx, plan)

	// Attempt to update the plan status before returning.
	return result, errors.Join(err, r.Status().Update(ctx, plan))
}

func (r *UpgradePlanReconciler) executePlan(ctx context.Context, upgradePlan *lifecyclev1alpha1.UpgradePlan) (ctrl.Result, error) {
	release, ok := r.Releases[upgradePlan.Spec.ReleaseVersion]
	if !ok {
		return ctrl.Result{}, fmt.Errorf("release manifest with version %v not found", upgradePlan.Spec.ReleaseVersion)
	}

	if len(upgradePlan.Status.Conditions) == 0 {
		condition := metav1.Condition{Type: lifecyclev1alpha1.KubernetesUpgradedCondition, Status: metav1.ConditionUnknown, Reason: lifecyclev1alpha1.UpgradePending, Message: "Kubernetes upgrade is not yet started"}
		meta.SetStatusCondition(&upgradePlan.Status.Conditions, condition)

		// Append OS and other components conditions here...
		return ctrl.Result{Requeue: true}, nil
	}

	// Upgrade OS here...

	if !meta.IsStatusConditionTrue(upgradePlan.Status.Conditions, lifecyclev1alpha1.KubernetesUpgradedCondition) {
		return r.reconcileKubernetes(ctx, upgradePlan, release.Components.Kubernetes.RKE2.Version)
	}

	// Upgrade rest of the components here...

	logger := log.FromContext(ctx)
	logger.Info("Upgrade completed successfully")

	return ctrl.Result{}, nil
}

func (r *UpgradePlanReconciler) recordCreatedPlan(upgradePlan *lifecyclev1alpha1.UpgradePlan, name, namespace string) {
	r.Recorder.Eventf(upgradePlan, corev1.EventTypeNormal, "PlanCreated", "Upgrade plan created: %s/%s", namespace, name)
}

func (r *UpgradePlanReconciler) reconcileKubernetes(ctx context.Context, upgradePlan *lifecyclev1alpha1.UpgradePlan, kubernetesVersion string) (ctrl.Result, error) {
	controlPlanePlan := &upgradecattlev1.Plan{}
	if err := r.Get(ctx, upgrade.KubernetesPlanKey(upgrade.ControlPlaneKey, kubernetesVersion), controlPlanePlan); err != nil {
		if !errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}

		controlPlanePlan = upgrade.KubernetesControlPlanePlan(kubernetesVersion)
		if err = ctrl.SetControllerReference(upgradePlan, controlPlanePlan, r.Scheme); err != nil {
			return ctrl.Result{}, fmt.Errorf("setting controller reference: %w", err)
		}

		if err = r.Create(ctx, controlPlanePlan); err != nil {
			return ctrl.Result{}, fmt.Errorf("creating control plane upgrade plan: %w", err)
		}

		r.recordCreatedPlan(upgradePlan, controlPlanePlan.Name, controlPlanePlan.Namespace)
		return ctrl.Result{Requeue: true}, nil
	}

	workerPlan := &upgradecattlev1.Plan{}
	if err := r.Get(ctx, upgrade.KubernetesPlanKey(upgrade.WorkersKey, kubernetesVersion), workerPlan); err != nil {
		if !errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}

		workerPlan = upgrade.KubernetesWorkerPlan(kubernetesVersion)
		if err = ctrl.SetControllerReference(upgradePlan, workerPlan, r.Scheme); err != nil {
			return ctrl.Result{}, fmt.Errorf("setting controller reference: %w", err)
		}

		if err = r.Create(ctx, workerPlan); err != nil {
			return ctrl.Result{}, fmt.Errorf("creating worker plan: %w", err)
		}

		r.recordCreatedPlan(upgradePlan, workerPlan.Name, workerPlan.Namespace)
		return ctrl.Result{Requeue: true}, nil
	}

	nodeList := &corev1.NodeList{}
	if err := r.List(ctx, nodeList); err != nil {
		return ctrl.Result{}, fmt.Errorf("listing nodes: %w", err)
	}

	selector, err := metav1.LabelSelectorAsSelector(controlPlanePlan.Spec.NodeSelector)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("parsing node selector: %w", err)
	}

	if !isKubernetesUpgraded(nodeList, selector, kubernetesVersion) {
		condition := metav1.Condition{Type: lifecyclev1alpha1.KubernetesUpgradedCondition, Status: metav1.ConditionFalse, Reason: lifecyclev1alpha1.UpgradeInProgress, Message: "Control plane nodes are being upgraded"}
		meta.SetStatusCondition(&upgradePlan.Status.Conditions, condition)

		return ctrl.Result{}, nil
	}

	selector, err = metav1.LabelSelectorAsSelector(workerPlan.Spec.NodeSelector)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("parsing node selector: %w", err)
	}

	if !isKubernetesUpgraded(nodeList, selector, kubernetesVersion) {
		condition := metav1.Condition{Type: lifecyclev1alpha1.KubernetesUpgradedCondition, Status: metav1.ConditionFalse, Reason: lifecyclev1alpha1.UpgradeInProgress, Message: "Worker nodes are being upgraded"}
		meta.SetStatusCondition(&upgradePlan.Status.Conditions, condition)
		return ctrl.Result{}, nil
	}

	condition := metav1.Condition{Type: lifecyclev1alpha1.KubernetesUpgradedCondition, Status: metav1.ConditionTrue, Reason: lifecyclev1alpha1.UpgradeSucceeded, Message: "All cluster nodes are upgraded"}
	meta.SetStatusCondition(&upgradePlan.Status.Conditions, condition)

	return ctrl.Result{Requeue: true}, nil
}

func isKubernetesUpgraded(nodeList *corev1.NodeList, selector labels.Selector, kubernetesVersion string) bool {
	for _, node := range nodeList.Items {
		if !selector.Matches(labels.Set(node.Labels)) {
			continue
		}

		var nodeReadyStatus corev1.ConditionStatus

		for _, condition := range node.Status.Conditions {
			if condition.Type == corev1.NodeReady {
				nodeReadyStatus = condition.Status
				break
			}
		}

		if nodeReadyStatus != corev1.ConditionTrue || node.Spec.Unschedulable || node.Status.NodeInfo.KubeletVersion != kubernetesVersion {
			// Upgrade is still in progress.
			// TODO: Adjust to looking at the `Complete` condition of the
			//  `plans.upgrade.cattle.io` resources once system-upgrade-controller v0.13.4 is released.
			return false
		}
	}

	return true
}

// SetupWithManager sets up the controller with the Manager.
func (r *UpgradePlanReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&lifecyclev1alpha1.UpgradePlan{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Owns(&upgradecattlev1.Plan{}, builder.WithPredicates(predicate.Funcs{
			UpdateFunc: func(e event.UpdateEvent) bool {
				// Upgrade plans are being constantly updated on every node change.
				// Ensure that the reconciliation only covers the scenarios
				// where the plans are no longer actively being applied to a node.
				return len(e.ObjectNew.(*upgradecattlev1.Plan).Status.Applying) == 0 &&
					len(e.ObjectOld.(*upgradecattlev1.Plan).Status.Applying) != 0
			},
		})).
		Complete(r)
}
