package controller

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	helmcattlev1 "github.com/k3s-io/helm-controller/pkg/apis/helm.cattle.io/v1"
	lifecyclev1alpha1 "github.com/suse-edge/upgrade-controller/api/v1alpha1"
	"github.com/suse-edge/upgrade-controller/internal/upgrade"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *UpgradePlanReconciler) reconcileKubernetes(
	ctx context.Context,
	upgradePlan *lifecyclev1alpha1.UpgradePlan,
	kubernetes *lifecyclev1alpha1.Kubernetes,
	nodeList *corev1.NodeList,
) (ctrl.Result, error) {
	nameSuffix := upgradePlan.Status.SUCNameSuffix

	k8sDistro, err := targetKubernetesDistribution(nodeList, kubernetes)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("identifying target kubernetes distribution: %w", err)
	}

	conditionType := lifecyclev1alpha1.KubernetesUpgradedCondition

	identifierLabels := upgrade.PlanIdentifierLabels(upgradePlan.Name, upgradePlan.Namespace)
	drainControlPlane, drainWorker := parseDrainOptions(nodeList, upgradePlan)
	controlPlanePlan := upgrade.KubernetesControlPlanePlan(nameSuffix, k8sDistro.Version, drainControlPlane, identifierLabels)
	if err = r.Get(ctx, client.ObjectKeyFromObject(controlPlanePlan), controlPlanePlan); err != nil {
		if !errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}

		setInProgressCondition(upgradePlan, conditionType, "Control plane nodes are being upgraded")
		return ctrl.Result{}, r.createObject(ctx, upgradePlan, controlPlanePlan)
	}

	nodes, err := findMatchingNodes(nodeList, controlPlanePlan.Spec.NodeSelector)
	if err != nil {
		return ctrl.Result{}, err
	}

	if !isKubernetesUpgraded(nodes, k8sDistro.Version) {
		setInProgressCondition(upgradePlan, conditionType, "Control plane nodes are being upgraded")
		return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
	} else if controlPlaneOnlyCluster(nodeList) {
		allUpgraded, waitingFor, err := r.getK8sCoreComponentsUpgradeStatus(ctx, k8sDistro.CoreComponents)
		if err != nil {
			return ctrl.Result{}, err
		}

		if !allUpgraded {
			msg := fmt.Sprintf("Waiting for %s core component to be upgraded", waitingFor)
			setInProgressCondition(upgradePlan, conditionType, msg)
			return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
		}

		setSuccessfulCondition(upgradePlan, conditionType, "All cluster nodes are upgraded")
		return ctrl.Result{Requeue: true}, nil
	}

	workerPlan := upgrade.KubernetesWorkerPlan(nameSuffix, k8sDistro.Version, drainWorker, identifierLabels)
	if err = r.Get(ctx, client.ObjectKeyFromObject(workerPlan), workerPlan); err != nil {
		if !errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}

		setInProgressCondition(upgradePlan, conditionType, "Worker nodes are being upgraded")
		return ctrl.Result{}, r.createObject(ctx, upgradePlan, workerPlan)
	}

	nodes, err = findMatchingNodes(nodeList, workerPlan.Spec.NodeSelector)
	if err != nil {
		return ctrl.Result{}, err
	}

	if !isKubernetesUpgraded(nodes, k8sDistro.Version) {
		setInProgressCondition(upgradePlan, conditionType, "Worker nodes are being upgraded")
		return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
	}

	allUpgraded, waitingFor, err := r.getK8sCoreComponentsUpgradeStatus(ctx, k8sDistro.CoreComponents)
	if err != nil {
		return ctrl.Result{}, err
	}

	if !allUpgraded {
		msg := fmt.Sprintf("Waiting for %s core component to be upgraded", waitingFor)
		setInProgressCondition(upgradePlan, conditionType, msg)
		return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
	}

	setSuccessfulCondition(upgradePlan, conditionType, "All cluster nodes are upgraded")
	return ctrl.Result{Requeue: true}, nil
}

func (r *UpgradePlanReconciler) getK8sCoreComponentsUpgradeStatus(ctx context.Context, core []lifecyclev1alpha1.CoreComponent) (allUpgraded bool, waitingFor string, err error) {
	for _, component := range core {
		upgraded, err := r.isK8sCoreComponentUpgraded(ctx, &component)
		if err != nil {
			if errors.IsNotFound(err) {
				// Not every component from the core list might be
				// on every cluster.
				continue
			}
			return false, "", fmt.Errorf("validating upgrade for component %s: %w", component.Name, err)
		}

		if !upgraded {
			return false, component.Name, nil
		}
	}

	return true, "", nil
}

func (r *UpgradePlanReconciler) isK8sCoreComponentUpgraded(ctx context.Context, component *lifecyclev1alpha1.CoreComponent) (bool, error) {
	switch component.Type {
	case lifecyclev1alpha1.HelmChartType:
		chart := &helmcattlev1.HelmChart{}
		if err := r.Get(ctx, upgrade.ChartNamespacedName(component.Name), chart); err != nil {
			return false, fmt.Errorf("getting %s helm chart: %w", component.Name, err)
		}

		chartJob := &batchv1.Job{}
		if err := r.Get(ctx, types.NamespacedName{Name: chart.Status.JobName, Namespace: chart.Namespace}, chartJob); err != nil {
			// If the HelmChart exists it must have a Job attatched to it.
			// If the Job is missing, this indicates a recreate of the Job has been issued
			// which can take some time on slower machines.
			return false, client.IgnoreNotFound(err)
		}

		isJobComplete := func(conditions []batchv1.JobCondition) bool {
			return slices.ContainsFunc(conditions, func(condition batchv1.JobCondition) bool {
				return condition.Status == corev1.ConditionTrue && condition.Type == batchv1.JobComplete
			})
		}

		if !isJobComplete(chartJob.Status.Conditions) {
			return false, nil
		}

		return compareChartReleaseWithVersion(chart.Name, component.Version)
	case lifecyclev1alpha1.DeploymentType:
		dep := &appsv1.Deployment{}
		if err := r.Get(ctx, types.NamespacedName{Name: component.Name, Namespace: upgrade.KubeSystemNamespace}, dep); err != nil {
			return false, fmt.Errorf("getting %s deployment: %w", component.Name, err)
		}

		isDeploymentAvailable := func(conditions []appsv1.DeploymentCondition) bool {
			return slices.ContainsFunc(conditions, func(condition appsv1.DeploymentCondition) bool {
				return condition.Status == corev1.ConditionTrue && condition.Type == appsv1.DeploymentAvailable
			})
		}

		if !isDeploymentAvailable(dep.Status.Conditions) {
			return false, nil
		}

		return upgrade.ContainsContainerImages(dep.Spec.Template.Spec.Containers, component.ConvertContainerSliceToMap(), false), nil
	default:
		return false, fmt.Errorf("unsupported component type: %s", component.Type)
	}
}

func targetKubernetesDistribution(nodeList *corev1.NodeList, kubernetes *lifecyclev1alpha1.Kubernetes) (*lifecyclev1alpha1.KubernetesDistribution, error) {
	if len(nodeList.Items) == 0 {
		return nil, fmt.Errorf("unable to determine current kubernetes distribution due to empty node list")
	}

	kubeletVersion := nodeList.Items[0].Status.NodeInfo.KubeletVersion

	switch {
	case strings.Contains(kubeletVersion, "k3s"):
		return &kubernetes.K3S, nil
	case strings.Contains(kubeletVersion, "rke2"):
		return &kubernetes.RKE2, nil
	default:
		return nil, fmt.Errorf("unsupported kubernetes distribution detected in version %s", kubeletVersion)
	}
}

func findMatchingNodes(nodeList *corev1.NodeList, nodeSelector *metav1.LabelSelector) ([]corev1.Node, error) {
	selector, err := metav1.LabelSelectorAsSelector(nodeSelector)
	if err != nil {
		return nil, fmt.Errorf("parsing node selector: %w", err)
	}

	var targetNodes []corev1.Node

	for _, node := range nodeList.Items {
		if selector.Matches(labels.Set(node.Labels)) {
			targetNodes = append(targetNodes, node)
		}
	}

	if len(targetNodes) == 0 {
		return nil, fmt.Errorf("none of the nodes match label selector: MatchLabels: %s, MatchExpressions: %s",
			nodeSelector.MatchLabels, nodeSelector.MatchExpressions)
	}

	return targetNodes, nil
}

func isKubernetesUpgraded(nodes []corev1.Node, kubernetesVersion string) bool {
	for _, node := range nodes {
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
