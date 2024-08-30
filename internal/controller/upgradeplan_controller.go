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
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var errUpgradeInProgress = errors.New("upgrade is currently in progress")

// UpgradePlanReconciler reconciles a UpgradePlan object
type UpgradePlanReconciler struct {
	client.Client
	Scheme               *runtime.Scheme
	Recorder             record.EventRecorder
	ServiceAccount       string
	ReleaseManifestImage string
}

// +kubebuilder:rbac:groups=lifecycle.suse.com,resources=upgradeplans,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=lifecycle.suse.com,resources=upgradeplans/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=lifecycle.suse.com,resources=upgradeplans/finalizers,verbs=update
// +kubebuilder:rbac:groups=upgrade.cattle.io,resources=plans,verbs=create;list;get;watch;delete
// +kubebuilder:rbac:groups="",resources=nodes,verbs=watch;list
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;delete;create;watch
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create
// +kubebuilder:rbac:groups=batch,resources=jobs/status,verbs=get
// +kubebuilder:rbac:groups=helm.cattle.io,resources=helmcharts,verbs=get;update;list;watch;create
// +kubebuilder:rbac:groups=helm.cattle.io,resources=helmcharts/status,verbs=get
// +kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get
// +kubebuilder:rbac:groups=lifecycle.suse.com,resources=releasemanifests,verbs=get;list;watch;create

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *UpgradePlanReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	plan := &lifecyclev1alpha1.UpgradePlan{}

	if err := r.Get(ctx, req.NamespacedName, plan); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	logger := log.FromContext(ctx)
	logger.Info("Reconciling UpgradePlan")

	if !plan.ObjectMeta.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(plan, lifecyclev1alpha1.UpgradePlanFinalizer) {
			return ctrl.Result{}, nil
		}

		if err := r.reconcileDelete(ctx, plan); err != nil {
			if errors.Is(err, errUpgradeInProgress) {
				// Requeue here is not necessary since the plan
				// will be reconciled again once the upgrade is complete.
				return ctrl.Result{}, nil
			}
			return ctrl.Result{}, err
		}

		controllerutil.RemoveFinalizer(plan, lifecyclev1alpha1.UpgradePlanFinalizer)
		return ctrl.Result{}, r.Update(ctx, plan)
	}

	if !controllerutil.ContainsFinalizer(plan, lifecyclev1alpha1.UpgradePlanFinalizer) {
		controllerutil.AddFinalizer(plan, lifecyclev1alpha1.UpgradePlanFinalizer)

		// add the finalizers and force a reconciliation
		return ctrl.Result{Requeue: true}, r.Update(ctx, plan)
	}

	result, err := r.reconcileNormal(ctx, plan)

	// Attempt to update the plan status before returning.
	return result, errors.Join(err, r.Status().Update(ctx, plan))
}

func (r *UpgradePlanReconciler) reconcileDelete(ctx context.Context, upgradePlan *lifecyclev1alpha1.UpgradePlan) error {
	sucPlans := &upgradecattlev1.PlanList{}

	if err := r.List(ctx, sucPlans, &client.ListOptions{
		Namespace: upgrade.SUCNamespace,
	}); err != nil {
		return fmt.Errorf("retrieving SUC plans: %w", err)
	}

	for _, plan := range sucPlans.Items {
		if plan.Annotations[upgrade.PlanNameAnnotation] != upgradePlan.Name ||
			plan.Annotations[upgrade.PlanNamespaceAnnotation] != upgradePlan.Namespace {
			continue
		}

		if len(plan.Status.Applying) != 0 {
			return errUpgradeInProgress
		}

		if err := r.Delete(ctx, &plan); err != nil {
			return fmt.Errorf("deleting SUC plan %s: %w", plan.Name, err)
		}
	}

	secrets := &corev1.SecretList{}

	if err := r.List(ctx, secrets, &client.ListOptions{
		Namespace: upgrade.SUCNamespace,
	}); err != nil {
		return fmt.Errorf("retrieving SUC secrets: %w", err)
	}

	for _, secret := range secrets.Items {
		if secret.Annotations[upgrade.PlanNameAnnotation] != upgradePlan.Name ||
			secret.Annotations[upgrade.PlanNamespaceAnnotation] != upgradePlan.Namespace {
			continue
		}

		if err := r.Delete(ctx, &secret); err != nil {
			return fmt.Errorf("deleting SUC secret %s: %w", secret.Name, err)
		}
	}

	return nil
}

func (r *UpgradePlanReconciler) reconcileNormal(ctx context.Context, upgradePlan *lifecyclev1alpha1.UpgradePlan) (ctrl.Result, error) {
	release, err := r.retrieveReleaseManifest(ctx, upgradePlan)
	if err != nil {
		if !errors.Is(err, errReleaseManifestNotFound) {
			return ctrl.Result{}, fmt.Errorf("retrieving release manifest: %w", err)
		}

		return ctrl.Result{}, r.createReleaseManifest(ctx, upgradePlan)
	}

	if upgradePlan.Status.ObservedGeneration != upgradePlan.Generation {
		suffix, err := upgrade.GenerateSuffix()
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("generating suffix: %w", err)
		}

		// validate that the generated suffix is not the same
		// as the current suffix
		if suffix == upgradePlan.Status.SUCNameSuffix {
			return ctrl.Result{Requeue: true}, nil
		}

		upgradePlan.Status.SUCNameSuffix = suffix
		upgradePlan.Status.ObservedGeneration = upgradePlan.Generation

		setPendingCondition(upgradePlan, lifecyclev1alpha1.OperatingSystemUpgradedCondition, upgradePendingMessage("OS"))
		setPendingCondition(upgradePlan, lifecyclev1alpha1.KubernetesUpgradedCondition, upgradePendingMessage("Kubernetes"))

		for _, chart := range release.Spec.Components.Workloads.Helm {
			setPendingCondition(upgradePlan, lifecyclev1alpha1.GetChartConditionType(chart.PrettyName), upgradePendingMessage(chart.PrettyName))
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
		if !isHelmUpgradeFinished(upgradePlan, lifecyclev1alpha1.GetChartConditionType(chart.PrettyName)) {
			return r.reconcileHelmChart(ctx, upgradePlan, &chart)
		}
	}

	logger := log.FromContext(ctx)
	logger.Info("Upgrade completed")

	return ctrl.Result{}, nil
}

func (r *UpgradePlanReconciler) createObject(ctx context.Context, upgradePlan *lifecyclev1alpha1.UpgradePlan, object client.Object) error {
	// Extract the kind first since the data of the object pointer is modified during creation.
	kind := object.GetObjectKind().GroupVersionKind().Kind

	if err := r.Create(ctx, object); err != nil {
		return err
	}

	reason := fmt.Sprintf("%sCreated", kind)
	r.Recorder.Eventf(upgradePlan, corev1.EventTypeNormal, reason,
		"%s created: %s/%s", kind, object.GetNamespace(), object.GetName())
	return nil
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

func parseDrainOptions(nodeList *corev1.NodeList, plan *lifecyclev1alpha1.UpgradePlan) (drainControlPlane bool, drainWorker bool) {
	var controlPlaneCounter, workerCounter int
	for _, node := range nodeList.Items {
		if node.Labels[upgrade.ControlPlaneLabel] != "true" {
			workerCounter++
		} else {
			controlPlaneCounter++
		}
	}

	switch {
	case controlPlaneCounter > 1 && workerCounter <= 1:
		drainControlPlane = true
		drainWorker = false
	case controlPlaneCounter == 1 && workerCounter > 1:
		drainControlPlane = false
		drainWorker = true
	case controlPlaneCounter <= 1 && workerCounter <= 1:
		drainControlPlane = false
		drainWorker = false
	default:
		drainControlPlane = true
		drainWorker = true
	}

	if plan.Spec.DisableDrain != nil {
		// If user has explicitly disabled control-plane drains
		if plan.Spec.DisableDrain.ControlPlane {
			drainControlPlane = false
		}

		// If user has explicitly disabled worker drains
		if plan.Spec.DisableDrain.Worker {
			drainWorker = false
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
	// Check whether the Job was created by the Upgrade Controller first
	requests := r.findUpgradePlanFromAnnotations(ctx, job)
	if len(requests) != 0 {
		return requests
	}

	// Check whether the Job was created by the Helm Controller
	jobLabels := job.GetLabels()
	chartName, ok := jobLabels[chart.Label]
	if !ok || chartName == "" {
		return []reconcile.Request{}
	}

	helmChart := &helmcattlev1.HelmChart{}
	if err := r.Get(ctx, upgrade.ChartNamespacedName(chartName), helmChart); err != nil {
		logger := log.FromContext(ctx)
		logger.Error(err, "failed to get helm chart")

		return []reconcile.Request{}
	}

	return r.findUpgradePlanFromAnnotations(ctx, helmChart)
}

func (r *UpgradePlanReconciler) findUpgradePlanFromAnnotations(_ context.Context, object client.Object) []reconcile.Request {
	annotations := object.GetAnnotations()

	planName, ok := annotations[upgrade.PlanNameAnnotation]
	if !ok || planName == "" {
		// Object is not managed by the Upgrade controller.
		return []reconcile.Request{}
	}

	planNamespace := annotations[upgrade.PlanNamespaceAnnotation]

	return []reconcile.Request{
		{NamespacedName: types.NamespacedName{Namespace: planNamespace, Name: planName}},
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
		Watches(&upgradecattlev1.Plan{}, handler.EnqueueRequestsFromMapFunc(r.findUpgradePlanFromAnnotations), builder.WithPredicates(predicate.Funcs{
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
			DeleteFunc: func(e event.DeleteEvent) bool {
				return false
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
		Watches(&corev1.Secret{}, handler.EnqueueRequestsFromMapFunc(r.findUpgradePlanFromAnnotations), builder.WithPredicates(predicate.Funcs{
			DeleteFunc: func(e event.DeleteEvent) bool {
				return false
			},
		})).
		Complete(r)
}
