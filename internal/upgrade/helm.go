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
