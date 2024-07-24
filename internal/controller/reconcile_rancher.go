package controller

import (
	"context"
	"fmt"
	"slices"

	helmcattlev1 "github.com/k3s-io/helm-controller/pkg/apis/helm.cattle.io/v1"
	lifecyclev1alpha1 "github.com/suse-edge/upgrade-controller/api/v1alpha1"
	"github.com/suse-edge/upgrade-controller/internal/upgrade"
	"github.com/suse-edge/upgrade-controller/pkg/release"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *UpgradePlanReconciler) reconcileRancher(ctx context.Context, upgradePlan *lifecyclev1alpha1.UpgradePlan, rancher *release.HelmChart) (ctrl.Result, error) {
	chart := &helmcattlev1.HelmChart{}

	if err := r.Get(ctx, upgrade.ChartNamespacedName(rancher.Name), chart); err != nil {
		if !errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}

		setSkippedCondition(upgradePlan, lifecyclev1alpha1.RancherUpgradedCondition, "Rancher installation is not found")
		return ctrl.Result{Requeue: true}, nil
	}

	if chart.Spec.Version != rancher.Version {
		setInProgressCondition(upgradePlan, lifecyclev1alpha1.RancherUpgradedCondition, "Rancher is being upgraded")

		return ctrl.Result{}, r.updateHelmChart(ctx, upgradePlan, chart, rancher)
	}

	job := &batchv1.Job{}
	if err := r.Get(ctx, types.NamespacedName{Name: chart.Status.JobName, Namespace: upgrade.ChartNamespace}, job); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	idx := slices.IndexFunc(job.Status.Conditions, func(condition batchv1.JobCondition) bool {
		return condition.Status == corev1.ConditionTrue &&
			(condition.Type == batchv1.JobComplete || condition.Type == batchv1.JobFailed)
	})

	if idx == -1 {
		// Upgrade job is still ongoing.
		return ctrl.Result{}, nil
	}

	condition := job.Status.Conditions[idx]

	switch condition.Type {
	case batchv1.JobComplete:
		setSuccessfulCondition(upgradePlan, lifecyclev1alpha1.RancherUpgradedCondition, "Rancher is upgraded")
	case batchv1.JobFailed:
		setFailedCondition(upgradePlan, lifecyclev1alpha1.RancherUpgradedCondition, fmt.Sprintf("Error occurred: %s", condition.Message))
	}

	return ctrl.Result{Requeue: true}, nil
}

func (r *UpgradePlanReconciler) updateHelmChart(ctx context.Context, upgradePlan *lifecyclev1alpha1.UpgradePlan, chart *helmcattlev1.HelmChart, releaseChart *release.HelmChart) error {
	backoffLimit := int32(6)

	if chart.Annotations == nil {
		chart.Annotations = map[string]string{}
	}
	chart.Annotations[upgrade.PlanAnnotation] = upgradePlan.Name
	chart.Spec.ChartContent = ""
	chart.Spec.Chart = releaseChart.Name
	chart.Spec.Version = releaseChart.Version
	chart.Spec.Repo = releaseChart.Repository
	chart.Spec.BackOffLimit = &backoffLimit

	return r.Update(ctx, chart)
}
