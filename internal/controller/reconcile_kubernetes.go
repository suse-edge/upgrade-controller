package controller

import (
	"context"
	"fmt"

	lifecyclev1alpha1 "github.com/suse-edge/upgrade-controller/api/v1alpha1"
	"github.com/suse-edge/upgrade-controller/internal/upgrade"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *UpgradePlanReconciler) reconcileKubernetes(ctx context.Context, upgradePlan *lifecyclev1alpha1.UpgradePlan, kubernetesVersion string) (ctrl.Result, error) {
	controlPlanePlan := upgrade.KubernetesControlPlanePlan(kubernetesVersion)
	if err := r.Get(ctx, client.ObjectKeyFromObject(controlPlanePlan), controlPlanePlan); err != nil {
		if !errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}

		return r.createPlan(ctx, upgradePlan, controlPlanePlan)
	}

	workerPlan := upgrade.KubernetesWorkerPlan(kubernetesVersion)
	if err := r.Get(ctx, client.ObjectKeyFromObject(workerPlan), workerPlan); err != nil {
		if !errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}

		return r.createPlan(ctx, upgradePlan, workerPlan)
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
