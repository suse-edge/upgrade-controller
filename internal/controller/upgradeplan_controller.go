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
	"slices"

	helmcattlev1 "github.com/k3s-io/helm-controller/pkg/apis/helm.cattle.io/v1"
	"github.com/k3s-io/helm-controller/pkg/controllers/chart"
	upgradecattlev1 "github.com/rancher/system-upgrade-controller/pkg/apis/upgrade.cattle.io/v1"
	lifecyclev1alpha1 "github.com/suse-edge/upgrade-controller/api/v1alpha1"
	"github.com/suse-edge/upgrade-controller/internal/upgrade"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// UpgradePlanReconciler reconciles a UpgradePlan object
type UpgradePlanReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=lifecycle.suse.com,resources=upgradeplans,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=lifecycle.suse.com,resources=upgradeplans/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=lifecycle.suse.com,resources=upgradeplans/finalizers,verbs=update
// +kubebuilder:rbac:groups=upgrade.cattle.io,resources=plans,verbs=create;list;get;watch
// +kubebuilder:rbac:groups="",resources=nodes,verbs=watch;list
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;delete;create;watch
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch
// +kubebuilder:rbac:groups=batch,resources=jobs/status,verbs=get
// +kubebuilder:rbac:groups=helm.cattle.io,resources=helmcharts,verbs=get;update;list;watch;create
// +kubebuilder:rbac:groups=helm.cattle.io,resources=helmcharts/status,verbs=get
// +kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get
// +kubebuilder:rbac:groups=lifecycle.suse.com,resources=releasemanifests,verbs=get;list;watch

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

func (r *UpgradePlanReconciler) getReleaseManifest(ctx context.Context, upgradePlan *lifecyclev1alpha1.UpgradePlan) (*lifecyclev1alpha1.ReleaseManifest, error) {
	manifests := &lifecyclev1alpha1.ReleaseManifestList{}
	listOpts := &client.ListOptions{
		Namespace: upgradePlan.Namespace,
	}
	if err := r.List(ctx, manifests, listOpts); err != nil {
		return nil, fmt.Errorf("listing release manifests in cluster: %w", err)
	}

	for _, manifest := range manifests.Items {
		if manifest.Spec.ReleaseVersion == upgradePlan.Spec.ReleaseVersion {
			return &manifest, nil
		}
	}

	return nil, fmt.Errorf("release manifest with version %s not found", upgradePlan.Spec.ReleaseVersion)
}

func (r *UpgradePlanReconciler) executePlan(ctx context.Context, upgradePlan *lifecyclev1alpha1.UpgradePlan) (ctrl.Result, error) {
	release, err := r.getReleaseManifest(ctx, upgradePlan)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("retrieving release manifest: %w", err)
	}

	if len(upgradePlan.Status.Conditions) == 0 {
		setPendingCondition(upgradePlan, lifecyclev1alpha1.OperatingSystemUpgradedCondition, upgradePendingMessage("OS"))
		setPendingCondition(upgradePlan, lifecyclev1alpha1.KubernetesUpgradedCondition, upgradePendingMessage("Kubernetes"))

		for _, chart := range release.Spec.Components.Workloads.Helm {
			setPendingCondition(upgradePlan, getChartConditionType(chart.PrettyName), upgradePendingMessage(chart.PrettyName))
		}

		return ctrl.Result{Requeue: true}, nil
	}

	switch {
	case !meta.IsStatusConditionTrue(upgradePlan.Status.Conditions, lifecyclev1alpha1.OperatingSystemUpgradedCondition):
		return r.reconcileOS(ctx, upgradePlan, release.Spec.ReleaseVersion, &release.Spec.Components.OperatingSystem)
	case !meta.IsStatusConditionTrue(upgradePlan.Status.Conditions, lifecyclev1alpha1.KubernetesUpgradedCondition):
		return r.reconcileKubernetes(ctx, upgradePlan, &release.Spec.Components.Kubernetes)
	}

	for _, chart := range release.Spec.Components.Workloads.Helm {
		if !isHelmUpgradeFinished(upgradePlan, getChartConditionType(chart.PrettyName)) {
			return r.reconcileHelmChart(ctx, upgradePlan, &chart)
		}
	}

	logger := log.FromContext(ctx)
	logger.Info("Upgrade completed")

	return ctrl.Result{}, nil
}

func (r *UpgradePlanReconciler) createSecret(ctx context.Context, upgradePlan *lifecyclev1alpha1.UpgradePlan, secret *corev1.Secret) error {
	if err := r.createObject(ctx, upgradePlan, secret); err != nil {
		return fmt.Errorf("creating secret: %w", err)
	}

	r.recordPlanEvent(upgradePlan, corev1.EventTypeNormal, "SecretCreated", fmt.Sprintf("Secret created: %s/%s", secret.Namespace, secret.Name))
	return nil
}

func (r *UpgradePlanReconciler) createPlan(ctx context.Context, upgradePlan *lifecyclev1alpha1.UpgradePlan, plan *upgradecattlev1.Plan) error {
	if err := r.createObject(ctx, upgradePlan, plan); err != nil {
		return fmt.Errorf("creating upgrade plan: %w", err)
	}

	r.recordPlanEvent(upgradePlan, corev1.EventTypeNormal, "PlanCreated", fmt.Sprintf("Upgrade plan created: %s/%s", plan.Namespace, plan.Name))
	return nil
}

func (r *UpgradePlanReconciler) createObject(ctx context.Context, upgradePlan *lifecyclev1alpha1.UpgradePlan, obj client.Object) error {
	if err := ctrl.SetControllerReference(upgradePlan, obj, r.Scheme); err != nil {
		return fmt.Errorf("setting controller reference: %w", err)
	}

	if err := r.Create(ctx, obj); err != nil {
		return fmt.Errorf("creating object: %w", err)
	}

	return nil
}

func (r *UpgradePlanReconciler) recordPlanEvent(upgradePlan *lifecyclev1alpha1.UpgradePlan, eventType, reason, msg string) {
	r.Recorder.Eventf(upgradePlan, eventType, reason, msg)
}

func isHelmUpgradeFinished(plan *lifecyclev1alpha1.UpgradePlan, conditionType string) bool {
	condition := meta.FindStatusCondition(plan.Status.Conditions, conditionType)

	if condition == nil {
		return false
	}

	if condition.Status == metav1.ConditionTrue {
		return true
	} else if condition.Status == metav1.ConditionFalse &&
		(condition.Reason == lifecyclev1alpha1.UpgradeSkipped || condition.Reason == lifecyclev1alpha1.UpgradeFailed) {
		return true
	}

	return false
}

func parseDrainOptions(plan *lifecyclev1alpha1.UpgradePlan) (drainControlPlane bool, drainWorker bool) {
	drainControlPlane = true
	drainWorker = true

	if plan.Spec.Drain != nil {
		if plan.Spec.Drain.ControlPlane != nil {
			drainControlPlane = *plan.Spec.Drain.ControlPlane
		}

		if plan.Spec.Drain.Worker != nil {
			drainWorker = *plan.Spec.Drain.Worker
		}
	}

	return drainControlPlane, drainWorker
}

func upgradePendingMessage(component string) string {
	return fmt.Sprintf("%s upgrade is not yet started", component)
}

type setCondition func(plan *lifecyclev1alpha1.UpgradePlan, conditionType string, message string)

func setPendingCondition(plan *lifecyclev1alpha1.UpgradePlan, conditionType, message string) {
	condition := metav1.Condition{Type: conditionType, Status: metav1.ConditionUnknown, Reason: lifecyclev1alpha1.UpgradePending, Message: message}
	meta.SetStatusCondition(&plan.Status.Conditions, condition)
}

func setErrorCondition(plan *lifecyclev1alpha1.UpgradePlan, conditionType, message string) {
	condition := metav1.Condition{Type: conditionType, Status: metav1.ConditionUnknown, Reason: lifecyclev1alpha1.UpgradeError, Message: message}
	meta.SetStatusCondition(&plan.Status.Conditions, condition)
}

func setInProgressCondition(plan *lifecyclev1alpha1.UpgradePlan, conditionType, message string) {
	condition := metav1.Condition{Type: conditionType, Status: metav1.ConditionFalse, Reason: lifecyclev1alpha1.UpgradeInProgress, Message: message}
	meta.SetStatusCondition(&plan.Status.Conditions, condition)
}

func setSuccessfulCondition(plan *lifecyclev1alpha1.UpgradePlan, conditionType, message string) {
	condition := metav1.Condition{Type: conditionType, Status: metav1.ConditionTrue, Reason: lifecyclev1alpha1.UpgradeSucceeded, Message: message}
	meta.SetStatusCondition(&plan.Status.Conditions, condition)
}

func setFailedCondition(plan *lifecyclev1alpha1.UpgradePlan, conditionType, message string) {
	condition := metav1.Condition{Type: conditionType, Status: metav1.ConditionFalse, Reason: lifecyclev1alpha1.UpgradeFailed, Message: message}
	meta.SetStatusCondition(&plan.Status.Conditions, condition)
}

func setSkippedCondition(plan *lifecyclev1alpha1.UpgradePlan, conditionType, message string) {
	condition := metav1.Condition{Type: conditionType, Status: metav1.ConditionFalse, Reason: lifecyclev1alpha1.UpgradeSkipped, Message: message}
	meta.SetStatusCondition(&plan.Status.Conditions, condition)
}

func (r *UpgradePlanReconciler) findUpgradePlanFromJob(ctx context.Context, job client.Object) []reconcile.Request {
	jobLabels := job.GetLabels()
	chartName, ok := jobLabels[chart.Label]
	if !ok || chartName == "" {
		// Job is not scheduled by the Helm controller.
		return []reconcile.Request{}
	}

	helmChart := &helmcattlev1.HelmChart{}
	if err := r.Get(ctx, upgrade.ChartNamespacedName(chartName), helmChart); err != nil {
		logger := log.FromContext(ctx)
		logger.Error(err, "failed to get helm chart")

		return []reconcile.Request{}
	}

	planName, ok := helmChart.Annotations[upgrade.PlanAnnotation]
	if !ok || planName == "" {
		// Helm chart is not managed by the Upgrade controller.
		return []reconcile.Request{}
	}

	return []reconcile.Request{
		{NamespacedName: upgrade.PlanNamespacedName(planName)},
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *UpgradePlanReconciler) SetupWithManager(mgr ctrl.Manager) error {
	definitionsGetter := clientset.NewForConfigOrDie(mgr.GetConfig()).ApiextensionsV1().CustomResourceDefinitions()

	helmChartKind := helmcattlev1.Kind(helmcattlev1.HelmChartResourceName)
	if _, err := definitionsGetter.Get(context.Background(), helmChartKind.String(), metav1.GetOptions{}); err != nil {
		return fmt.Errorf("verifying Helm Controller installation: %w", err)
	}

	sucPlanKind := upgradecattlev1.Kind(upgradecattlev1.PlanResourceName)
	if _, err := definitionsGetter.Get(context.Background(), sucPlanKind.String(), metav1.GetOptions{}); err != nil {
		return fmt.Errorf("verifying System Upgrade Controller installation: %w", err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&lifecyclev1alpha1.UpgradePlan{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Owns(&upgradecattlev1.Plan{}, builder.WithPredicates(predicate.Funcs{
			CreateFunc: func(e event.CreateEvent) bool {
				return false
			},
			UpdateFunc: func(e event.UpdateEvent) bool {
				// Upgrade plans are being constantly updated on every node change.
				// Ensure that the reconciliation only covers the scenarios
				// where the plans are no longer actively being applied to a node.
				return len(e.ObjectNew.(*upgradecattlev1.Plan).Status.Applying) == 0 &&
					len(e.ObjectOld.(*upgradecattlev1.Plan).Status.Applying) != 0
			},
		})).
		Watches(&batchv1.Job{}, handler.EnqueueRequestsFromMapFunc(r.findUpgradePlanFromJob), builder.WithPredicates(predicate.Funcs{
			CreateFunc: func(e event.CreateEvent) bool {
				return false
			},
			UpdateFunc: func(e event.UpdateEvent) bool {
				// Only requeue an upgrade plan when a respective job finishes.
				isJobFinished := func(conditions []batchv1.JobCondition) bool {
					return slices.ContainsFunc(conditions, func(condition batchv1.JobCondition) bool {
						return condition.Status == corev1.ConditionTrue &&
							(condition.Type == batchv1.JobComplete || condition.Type == batchv1.JobFailed)
					})
				}

				return isJobFinished(e.ObjectNew.(*batchv1.Job).Status.Conditions) &&
					!isJobFinished(e.ObjectOld.(*batchv1.Job).Status.Conditions)
			},
			DeleteFunc: func(e event.DeleteEvent) bool {
				return false
			},
			GenericFunc: func(e event.GenericEvent) bool {
				return false
			},
		})).
		Owns(&corev1.Secret{}).
		Complete(r)
}
