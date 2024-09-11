package controller

import (
	"context"
	"fmt"
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

func (r *UpgradePlanReconciler) reconcileOS(
	ctx context.Context,
	upgradePlan *lifecyclev1alpha1.UpgradePlan,
	releaseVersion string,
	releaseOS *lifecyclev1alpha1.OperatingSystem,
	nodeList *corev1.NodeList,
) (ctrl.Result, error) {
	identifierAnnotations := upgrade.PlanIdentifierAnnotations(upgradePlan.Name, upgradePlan.Namespace)
	nameSuffix := upgradePlan.Status.SUCNameSuffix

	secret, err := upgrade.OSUpgradeSecret(nameSuffix, releaseOS, identifierAnnotations)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("generating OS upgrade secret: %w", err)
	}

	if err = r.Get(ctx, client.ObjectKeyFromObject(secret), secret); err != nil {
		if !errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, r.createObject(ctx, upgradePlan, secret)
	}

	conditionType := lifecyclev1alpha1.OperatingSystemUpgradedCondition

	drainControlPlane, drainWorker := parseDrainOptions(nodeList, upgradePlan)
	controlPlanePlan := upgrade.OSControlPlanePlan(nameSuffix, releaseVersion, secret.Name, releaseOS, drainControlPlane, identifierAnnotations)
	if err = r.Get(ctx, client.ObjectKeyFromObject(controlPlanePlan), controlPlanePlan); err != nil {
		if !errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}

		setInProgressCondition(upgradePlan, conditionType, "Control plane nodes are being upgraded")
		return ctrl.Result{}, r.createObject(ctx, upgradePlan, controlPlanePlan)
	}

	selector, err := metav1.LabelSelectorAsSelector(controlPlanePlan.Spec.NodeSelector)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("parsing node selector: %w", err)
	}

	if !isOSUpgraded(nodeList, selector, releaseOS.PrettyName) {
		setInProgressCondition(upgradePlan, conditionType, "Control plane nodes are being upgraded")
		return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
	} else if controlPlaneOnlyCluster(nodeList) {
		setSuccessfulCondition(upgradePlan, conditionType, "All cluster nodes are upgraded")
		return ctrl.Result{Requeue: true}, nil
	}

	workerPlan := upgrade.OSWorkerPlan(nameSuffix, releaseVersion, secret.Name, releaseOS, drainWorker, identifierAnnotations)
	if err = r.Get(ctx, client.ObjectKeyFromObject(workerPlan), workerPlan); err != nil {
		if !errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}

		setInProgressCondition(upgradePlan, conditionType, "Worker nodes are being upgraded")
		return ctrl.Result{}, r.createObject(ctx, upgradePlan, workerPlan)
	}

	selector, err = metav1.LabelSelectorAsSelector(workerPlan.Spec.NodeSelector)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("parsing node selector: %w", err)
	}

	if !isOSUpgraded(nodeList, selector, releaseOS.PrettyName) {
		setInProgressCondition(upgradePlan, conditionType, "Worker nodes are being upgraded")
		return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
	}

	setSuccessfulCondition(upgradePlan, conditionType, "All cluster nodes are upgraded")
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

func findUnsupportedNodes(nodeList *corev1.NodeList) []string {
	var unsupported []string

	for _, node := range nodeList.Items {
		if _, ok := lifecyclev1alpha1.SupportedArchitectures[node.Status.NodeInfo.Architecture]; !ok {
			unsupported = append(unsupported, node.Name)
		}
	}

	return unsupported
}
