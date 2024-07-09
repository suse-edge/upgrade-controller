package plan

import (
	upgradecattlev1 "github.com/rancher/system-upgrade-controller/pkg/apis/upgrade.cattle.io/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Option func(plan *upgradecattlev1.Plan)

func New(name string, opts ...Option) *upgradecattlev1.Plan {
	plan := baseUpgradePlan(name)

	for _, opt := range opts {
		opt(plan)
	}

	return plan
}

func baseUpgradePlan(name string) *upgradecattlev1.Plan {
	const (
		kind               = "Plan"
		apiVersion         = "upgrade.cattle.io/v1"
		namespace          = "cattle-system"
		serviceAccountName = "system-upgrade-controller"
	)

	plan := &upgradecattlev1.Plan{
		TypeMeta: metav1.TypeMeta{
			Kind:       kind,
			APIVersion: apiVersion,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: upgradecattlev1.PlanSpec{
			ServiceAccountName: serviceAccountName,
		},
	}

	return plan
}

func WithUpgradeSpec(spec *upgradecattlev1.ContainerSpec) Option {
	return func(plan *upgradecattlev1.Plan) {
		plan.Spec.Upgrade = spec
	}
}

func WithPrepareSpec(spec *upgradecattlev1.ContainerSpec) Option {
	return func(plan *upgradecattlev1.Plan) {
		plan.Spec.Prepare = spec
	}
}

func WithLabels(labels map[string]string) Option {
	return func(plan *upgradecattlev1.Plan) {
		plan.Labels = labels
	}
}

func WithConcurrency(concurrency int64) Option {
	return func(plan *upgradecattlev1.Plan) {
		plan.Spec.Concurrency = concurrency
	}
}

func WithNodeSelector(requirements []metav1.LabelSelectorRequirement) Option {
	return func(plan *upgradecattlev1.Plan) {
		plan.Spec.NodeSelector = &metav1.LabelSelector{
			MatchExpressions: requirements,
		}
	}
}

func WithVersion(version string) Option {
	return func(plan *upgradecattlev1.Plan) {
		plan.Spec.Version = version
	}
}

func WithTolerations(tolerations []corev1.Toleration) Option {
	return func(plan *upgradecattlev1.Plan) {
		plan.Spec.Tolerations = tolerations
	}
}

func WithCordon(cordon bool) Option {
	return func(plan *upgradecattlev1.Plan) {
		plan.Spec.Cordon = cordon
	}
}

func WithDrain(spec *upgradecattlev1.DrainSpec) Option {
	return func(plan *upgradecattlev1.Plan) {
		plan.Spec.Drain = spec
	}
}
