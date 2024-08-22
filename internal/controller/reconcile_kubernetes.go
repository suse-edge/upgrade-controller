package controller

import (
	"context"
	"fmt"
	"strings"
	"time"

	lifecyclev1alpha1 "github.com/suse-edge/upgrade-controller/api/v1alpha1"
	"github.com/suse-edge/upgrade-controller/internal/upgrade"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *UpgradePlanReconciler) reconcileKubernetes(ctx context.Context, upgradePlan *lifecyclev1alpha1.UpgradePlan, kubernetes *lifecyclev1alpha1.Kubernetes) (ctrl.Result, error) {
	nodeList := &corev1.NodeList{}
	if err := r.List(ctx, nodeList); err != nil {
		return ctrl.Result{}, fmt.Errorf("listing nodes: %w", err)
	}

	kubernetesVersion, err := targetKubernetesVersion(nodeList, kubernetes)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("identifying target kubernetes version: %w", err)
	}

	identifierAnnotations := upgrade.PlanIdentifierAnnotations(upgradePlan.Name, upgradePlan.Namespace)
	drainControlPlane, drainWorker := isDrainPossible(nodeList, upgradePlan)
	controlPlanePlan := upgrade.KubernetesControlPlanePlan(kubernetesVersion, drainControlPlane, identifierAnnotations)
	if err = r.Get(ctx, client.ObjectKeyFromObject(controlPlanePlan), controlPlanePlan); err != nil {
		if !errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}

		setInProgressCondition(upgradePlan, lifecyclev1alpha1.KubernetesUpgradedCondition, "Control plane nodes are being upgraded")
		return ctrl.Result{}, r.createObject(ctx, upgradePlan, controlPlanePlan)
	}

	selector, err := metav1.LabelSelectorAsSelector(controlPlanePlan.Spec.NodeSelector)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("parsing node selector: %w", err)
	}

	if !isKubernetesUpgraded(nodeList, selector, kubernetesVersion) {
		setInProgressCondition(upgradePlan, lifecyclev1alpha1.KubernetesUpgradedCondition, "Control plane nodes are being upgraded")
		return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
	} else if controlPlaneOnlyCluster(nodeList) {
		setSuccessfulCondition(upgradePlan, lifecyclev1alpha1.KubernetesUpgradedCondition, "All cluster nodes are upgraded")
		return ctrl.Result{Requeue: true}, nil
	}

	workerPlan := upgrade.KubernetesWorkerPlan(kubernetesVersion, drainWorker, identifierAnnotations)
	if err = r.Get(ctx, client.ObjectKeyFromObject(workerPlan), workerPlan); err != nil {
		if !errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}

		setInProgressCondition(upgradePlan, lifecyclev1alpha1.KubernetesUpgradedCondition, "Worker nodes are being upgraded")
		return ctrl.Result{}, r.createObject(ctx, upgradePlan, workerPlan)
	}

	selector, err = metav1.LabelSelectorAsSelector(workerPlan.Spec.NodeSelector)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("parsing node selector: %w", err)
	}

	if !isKubernetesUpgraded(nodeList, selector, kubernetesVersion) {
		setInProgressCondition(upgradePlan, lifecyclev1alpha1.KubernetesUpgradedCondition, "Worker nodes are being upgraded")
		return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
	}

	setSuccessfulCondition(upgradePlan, lifecyclev1alpha1.KubernetesUpgradedCondition, "All cluster nodes are upgraded")
	return ctrl.Result{Requeue: true}, nil
}

func targetKubernetesVersion(nodeList *corev1.NodeList, kubernetes *lifecyclev1alpha1.Kubernetes) (string, error) {
	if len(nodeList.Items) == 0 {
		return "", fmt.Errorf("unable to determine current kubernetes version due to empty node list")
	}

	kubeletVersion := nodeList.Items[0].Status.NodeInfo.KubeletVersion

	switch {
	case strings.Contains(kubeletVersion, "k3s"):
		return kubernetes.K3S.Version, nil
	case strings.Contains(kubeletVersion, "rke2"):
		return kubernetes.RKE2.Version, nil
	default:
		return "", fmt.Errorf("upgrading from kubernetes version %s is not supported", kubeletVersion)
	}
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

func controlPlaneOnlyCluster(nodeList *corev1.NodeList) bool {
	for _, node := range nodeList.Items {
		if node.Labels[upgrade.ControlPlaneLabel] != "true" {
			return false
		}
	}

	return true
}
