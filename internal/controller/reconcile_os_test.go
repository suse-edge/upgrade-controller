package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	lifecyclev1alpha1 "github.com/suse-edge/upgrade-controller/api/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func TestFindUnsupportedNodes(t *testing.T) {
	supportedArchitectures := lifecyclev1alpha1.SupportedArchitectures(
		[]lifecyclev1alpha1.Arch{lifecyclev1alpha1.ArchTypeARM, lifecyclev1alpha1.ArchTypeX86})

	nodes := &corev1.NodeList{
		Items: []corev1.Node{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "node1"},
				Status:     corev1.NodeStatus{NodeInfo: corev1.NodeSystemInfo{Architecture: "x86"}},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "node2"},
				Status:     corev1.NodeStatus{NodeInfo: corev1.NodeSystemInfo{Architecture: "arm64"}},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "node3"},
				Status:     corev1.NodeStatus{NodeInfo: corev1.NodeSystemInfo{Architecture: "aarch64"}},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "node4"},
				Status:     corev1.NodeStatus{NodeInfo: corev1.NodeSystemInfo{Architecture: "risc-v"}},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "node5"},
				Status:     corev1.NodeStatus{NodeInfo: corev1.NodeSystemInfo{Architecture: "amd64"}},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "node6"},
				Status:     corev1.NodeStatus{NodeInfo: corev1.NodeSystemInfo{Architecture: "x86_64"}},
			},
		}}

	assert.Equal(t, []string{"node1", "node4"}, findUnsupportedNodes(nodes, supportedArchitectures))
}

func TestIsOSUpgraded(t *testing.T) {
	const osPrettyName = "SUSE Linux Micro 6.0"

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
							NodeInfo:   corev1.NodeSystemInfo{OSImage: "SUSE Linux Micro 6.0"}},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Labels: nodeLabels},
						Spec:       corev1.NodeSpec{Unschedulable: false},
						Status: corev1.NodeStatus{
							Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}},
							NodeInfo:   corev1.NodeSystemInfo{OSImage: "SUSE Linux Micro 6.0"}},
					},
					{
						Status: corev1.NodeStatus{
							Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}},
							NodeInfo:   corev1.NodeSystemInfo{OSImage: "SUSE Linux Enterprise Micro 5.5"}},
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
							NodeInfo:   corev1.NodeSystemInfo{OSImage: "SUSE Linux Micro 6.0"}},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Labels: nodeLabels},
						Spec:       corev1.NodeSpec{Unschedulable: false},
						Status: corev1.NodeStatus{
							Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}},
							NodeInfo:   corev1.NodeSystemInfo{OSImage: "SUSE Linux Micro 6.0"}},
					},
					{
						Status: corev1.NodeStatus{
							Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}},
							NodeInfo:   corev1.NodeSystemInfo{OSImage: "SUSE Linux Enterprise Micro 5.5"}},
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
							NodeInfo:   corev1.NodeSystemInfo{OSImage: "SUSE Linux Micro 6.0"}},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Labels: nodeLabels},
						Spec:       corev1.NodeSpec{Unschedulable: false},
						Status: corev1.NodeStatus{
							Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}},
							NodeInfo:   corev1.NodeSystemInfo{OSImage: "SUSE Linux Micro 6.0"}},
					},
					{
						Status: corev1.NodeStatus{
							Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}},
							NodeInfo:   corev1.NodeSystemInfo{OSImage: "SUSE Linux Enterprise Micro 5.5"}},
					},
				},
			},
			selector:        labels.SelectorFromSet(nodeLabels),
			expectedUpgrade: false,
		},
		{
			name: "Matching node on older OS",
			nodes: &corev1.NodeList{
				Items: []corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{Labels: nodeLabels},
						Spec:       corev1.NodeSpec{Unschedulable: false},
						Status: corev1.NodeStatus{
							Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}},
							NodeInfo:   corev1.NodeSystemInfo{OSImage: "SUSE Linux Micro Micro 5.5"}},
					},
					{
						ObjectMeta: metav1.ObjectMeta{Labels: nodeLabels},
						Spec:       corev1.NodeSpec{Unschedulable: false},
						Status: corev1.NodeStatus{
							Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}},
							NodeInfo:   corev1.NodeSystemInfo{OSImage: "SUSE Linux Micro 6.0"}},
					},
					{
						Status: corev1.NodeStatus{
							Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}},
							NodeInfo:   corev1.NodeSystemInfo{OSImage: "SUSE Linux Enterprise Micro 5.5"}},
					},
				},
			},
			selector:        labels.SelectorFromSet(nodeLabels),
			expectedUpgrade: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expectedUpgrade, isOSUpgraded(test.nodes, test.selector, osPrettyName))
		})
	}
}
