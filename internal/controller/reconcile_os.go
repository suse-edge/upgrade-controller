package controller

import (
	"context"
	"fmt"

	lifecyclev1alpha1 "github.com/suse-edge/upgrade-controller/api/v1alpha1"
	"github.com/suse-edge/upgrade-controller/internal/upgrade"
	"github.com/suse-edge/upgrade-controller/pkg/release"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

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

	drainControlPlanes, drainWorkers := parseDrainOptions(upgradePlan)
	controlPlanePlan := upgrade.OSControlPlanePlan(release.ReleaseVersion, secret.Name, &release.Components.OperatingSystem, drainControlPlanes)
	if err = r.Get(ctx, client.ObjectKeyFromObject(controlPlanePlan), controlPlanePlan); err != nil {
		if !errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}

		setInProgressCondition(upgradePlan, lifecyclev1alpha1.OperatingSystemUpgradedCondition, "Control plane nodes are being upgraded")
		return ctrl.Result{}, r.createPlan(ctx, upgradePlan, controlPlanePlan)
	}

	selector, err := metav1.LabelSelectorAsSelector(controlPlanePlan.Spec.NodeSelector)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("parsing node selector: %w", err)
	}

	nodeList := &corev1.NodeList{}
	if err := r.List(ctx, nodeList); err != nil {
		return ctrl.Result{}, fmt.Errorf("listing nodes: %w", err)
	}

	if !isOSUpgraded(nodeList, selector, release.Components.OperatingSystem.PrettyName) {
		setInProgressCondition(upgradePlan, lifecyclev1alpha1.OperatingSystemUpgradedCondition, "Control plane nodes are being upgraded")
		return ctrl.Result{}, nil
	} else if controlPlaneOnlyCluster(nodeList) {
		setSuccessfulCondition(upgradePlan, lifecyclev1alpha1.OperatingSystemUpgradedCondition, "All cluster nodes are upgraded")
		return ctrl.Result{Requeue: true}, nil
	}

	workerPlan := upgrade.OSWorkerPlan(release.ReleaseVersion, secret.Name, &release.Components.OperatingSystem, drainWorkers)
	if err = r.Get(ctx, client.ObjectKeyFromObject(workerPlan), workerPlan); err != nil {
		if !errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}

		setInProgressCondition(upgradePlan, lifecyclev1alpha1.OperatingSystemUpgradedCondition, "Worker nodes are being upgraded")
		return ctrl.Result{}, r.createPlan(ctx, upgradePlan, workerPlan)
	}

	selector, err = metav1.LabelSelectorAsSelector(workerPlan.Spec.NodeSelector)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("parsing node selector: %w", err)
	}

	if !isOSUpgraded(nodeList, selector, release.Components.OperatingSystem.PrettyName) {
		setInProgressCondition(upgradePlan, lifecyclev1alpha1.OperatingSystemUpgradedCondition, "Worker nodes are being upgraded")
		return ctrl.Result{}, nil
	}

	setSuccessfulCondition(upgradePlan, lifecyclev1alpha1.OperatingSystemUpgradedCondition, "All cluster nodes are upgraded")
	return ctrl.Result{Requeue: true}, nil
}

func isOSUpgraded(nodeList *corev1.NodeList, selector labels.Selector, osPrettyName string) bool {
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

		if nodeReadyStatus != corev1.ConditionTrue || node.Spec.Unschedulable || node.Status.NodeInfo.OSImage != osPrettyName {
			// Upgrade is still in progress.
			// TODO: Adjust to looking at the `Complete` condition of the
			//  `plans.upgrade.cattle.io` resources once system-upgrade-controller v0.13.4 is released.
			return false
		}
	}

	return true
}
