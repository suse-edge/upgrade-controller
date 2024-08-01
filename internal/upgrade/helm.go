package upgrade

import "k8s.io/apimachinery/pkg/types"

const (
	ChartNamespace = "kube-system"
)

func ChartNamespacedName(chart string) types.NamespacedName {
	return types.NamespacedName{
		Name:      chart,
		Namespace: ChartNamespace,
	}
}

type HelmChartState int

const (
	ChartStateUnknown HelmChartState = iota
	ChartStateNotInstalled
	ChartStateVersionAlreadyInstalled
	ChartStateInProgress
	ChartStateFailed
	ChartStateSucceeded
)

func (s HelmChartState) Message() string {
	switch s {
	case ChartStateUnknown:
		return "Chart state is unknown"
	case ChartStateNotInstalled:
		return "Chart is not installed"
	case ChartStateVersionAlreadyInstalled:
		return "Chart version is already installed"
	case ChartStateInProgress:
		return "Chart upgrade is in progress"
	case ChartStateFailed:
		return "Chart upgrade failed"
	case ChartStateSucceeded:
		return "Chart upgrade succeeded"
	default:
		return ""
	}
}
