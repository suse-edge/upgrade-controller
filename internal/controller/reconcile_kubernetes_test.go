package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func TestIsKubernetesUpgraded(t *testing.T) {
	const kubernetesVersion = "v1.30.3+k3s1"

	nodeLabels := map[string]string{
		"node-x": "z",
	}

	tests := []struct {
		name            string
		nodes           *corev1.NodeList
		selector        labels.Selector
		expectedUpgrade bool
	}{
		{
			name: "All matching nodes upgraded",
			nodes: &corev1.NodeList{
				Items: []corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{Labels: nodeLabels},
						Spec:       corev1.NodeSpec{Unschedulable: false},
						Status: corev1.NodeStatus{
							Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}},
							NodeInfo:   corev1.NodeSystemInfo{KubeletVersion: "v1.30.3+k3s1"}},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Labels: nodeLabels},
						Spec:       corev1.NodeSpec{Unschedulable: false},
						Status: corev1.NodeStatus{
							Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}},
							NodeInfo:   corev1.NodeSystemInfo{KubeletVersion: "v1.30.3+k3s1"}},
					},
					{
						Status: corev1.NodeStatus{
							Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}},
							NodeInfo:   corev1.NodeSystemInfo{KubeletVersion: "v1.28.12+k3s1"}},
					},
				},
			},
			selector:        labels.SelectorFromSet(nodeLabels),
			expectedUpgrade: true,
		},
		{
			name: "Unschedulable matching node",
			nodes: &corev1.NodeList{
				Items: []corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{Labels: nodeLabels},
						Spec:       corev1.NodeSpec{Unschedulable: true},
						Status: corev1.NodeStatus{
							Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}},
							NodeInfo:   corev1.NodeSystemInfo{KubeletVersion: "v1.30.3+k3s1"}},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Labels: nodeLabels},
						Spec:       corev1.NodeSpec{Unschedulable: false},
						Status: corev1.NodeStatus{
							Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}},
							NodeInfo:   corev1.NodeSystemInfo{KubeletVersion: "v1.30.3+k3s1"}},
					},
					{
						Status: corev1.NodeStatus{
							Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}},
							NodeInfo:   corev1.NodeSystemInfo{KubeletVersion: "v1.28.12+k3s1"}},
					},
				},
			},
			selector:        labels.SelectorFromSet(nodeLabels),
			expectedUpgrade: false,
		},
		{
			name: "Not ready matching node",
			nodes: &corev1.NodeList{
				Items: []corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{Labels: nodeLabels},
						Spec:       corev1.NodeSpec{Unschedulable: false},
						Status: corev1.NodeStatus{
							Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionFalse}},
							NodeInfo:   corev1.NodeSystemInfo{KubeletVersion: "v1.30.3+k3s1"}},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Labels: nodeLabels},
						Spec:       corev1.NodeSpec{Unschedulable: false},
						Status: corev1.NodeStatus{
							Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}},
							NodeInfo:   corev1.NodeSystemInfo{KubeletVersion: "v1.30.3+k3s1"}},
					},
					{
						Status: corev1.NodeStatus{
							Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}},
							NodeInfo:   corev1.NodeSystemInfo{KubeletVersion: "v1.28.12+k3s1"}},
					},
				},
			},
			selector:        labels.SelectorFromSet(nodeLabels),
			expectedUpgrade: false,
		},
		{
			name: "Matching node on older Kubernetes version",
			nodes: &corev1.NodeList{
				Items: []corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{Labels: nodeLabels},
						Spec:       corev1.NodeSpec{Unschedulable: false},
						Status: corev1.NodeStatus{
							Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}},
							NodeInfo:   corev1.NodeSystemInfo{KubeletVersion: "v1.28.12+k3s1"}},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Labels: nodeLabels},
						Spec:       corev1.NodeSpec{Unschedulable: false},
						Status: corev1.NodeStatus{
							Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}},
							NodeInfo:   corev1.NodeSystemInfo{KubeletVersion: "v1.30.3+k3s1"}},
					},
					{
						Status: corev1.NodeStatus{
							Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}},
							NodeInfo:   corev1.NodeSystemInfo{KubeletVersion: "v1.28.12+k3s1"}},
					},
				},
			},
			selector:        labels.SelectorFromSet(nodeLabels),
			expectedUpgrade: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expectedUpgrade, isKubernetesUpgraded(test.nodes, test.selector, kubernetesVersion))
		})
	}
}

func TestControlPlaneOnlyCluster(t *testing.T) {
	assert.True(t, controlPlaneOnlyCluster(&corev1.NodeList{
		Items: []corev1.Node{
			{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"node-role.kubernetes.io/control-plane": "true"}}},
			{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"node-role.kubernetes.io/control-plane": "true"}}},
		},
	}))

	assert.False(t, controlPlaneOnlyCluster(&corev1.NodeList{
		Items: []corev1.Node{
			{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"node-role.kubernetes.io/control-plane": "true"}}},
			{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"node-role.kubernetes.io/control-plane": "false"}}},
		},
	}))

	assert.False(t, controlPlaneOnlyCluster(&corev1.NodeList{
		Items: []corev1.Node{
			{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"node-role.kubernetes.io/control-plane": "true"}}},
			{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{}}},
		},
	}))

	assert.False(t, controlPlaneOnlyCluster(&corev1.NodeList{
		Items: []corev1.Node{
			{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{}}},
		},
	}))
}
