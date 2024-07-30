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

func (r *UpgradePlanReconciler) reconcileLonghorn(ctx context.Context, upgradePlan *lifecyclev1alpha1.UpgradePlan, longhorn *release.HelmChart) (ctrl.Result, error) {
	helmRelease, err := retrieveHelmRelease(longhorn.Name)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("retrieving helm release: %w", err)
	}

	if helmRelease == nil {
		setSkippedCondition(upgradePlan, lifecyclev1alpha1.LonghornUpgradedCondition, "Longhorn installation is not found")
		return ctrl.Result{Requeue: true}, nil
	}

	chart := &helmcattlev1.HelmChart{}

	if err = r.Get(ctx, upgrade.ChartNamespacedName(helmRelease.Name), chart); err != nil {
		if !errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}

		if helmRelease.Chart.Metadata.Version == longhorn.Version {
			setSkippedCondition(upgradePlan, lifecyclev1alpha1.LonghornUpgradedCondition, versionAlreadyInstalledMessage(upgradePlan))
			return ctrl.Result{Requeue: true}, nil
		}

		setInProgressCondition(upgradePlan, lifecyclev1alpha1.LonghornUpgradedCondition, "Longhorn is being upgraded")
		return ctrl.Result{}, r.createHelmChart(ctx, upgradePlan, helmRelease, longhorn)
	}

	if chart.Spec.Version != longhorn.Version {
		setInProgressCondition(upgradePlan, lifecyclev1alpha1.LonghornUpgradedCondition, "Longhorn is being upgraded")
		return ctrl.Result{}, r.updateHelmChart(ctx, upgradePlan, chart, longhorn)
	}

	releaseVersion := chart.Annotations[upgrade.ReleaseAnnotation]
	if releaseVersion != upgradePlan.Spec.ReleaseVersion {
		setSkippedCondition(upgradePlan, lifecyclev1alpha1.LonghornUpgradedCondition, versionAlreadyInstalledMessage(upgradePlan))
		return ctrl.Result{Requeue: true}, nil
	}

	job := &batchv1.Job{}
	if err = r.Get(ctx, types.NamespacedName{Name: chart.Status.JobName, Namespace: upgrade.ChartNamespace}, job); err != nil {
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
		setSuccessfulCondition(upgradePlan, lifecyclev1alpha1.LonghornUpgradedCondition, "Longhorn is upgraded")
	case batchv1.JobFailed:
		setFailedCondition(upgradePlan, lifecyclev1alpha1.LonghornUpgradedCondition, fmt.Sprintf("Error occurred: %s", condition.Message))
	}

	return ctrl.Result{Requeue: true}, nil
}
