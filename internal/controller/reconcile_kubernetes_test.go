package controller

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	lifecyclev1alpha1 "github.com/suse-edge/upgrade-controller/api/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestFindMatchingNodes(t *testing.T) {
	nodeLabels := map[string]string{
		"node-x": "z",
	}
	nodeSelector := &metav1.LabelSelector{
		MatchLabels: nodeLabels,
	}

	tests := []struct {
		name          string
		nodeList      *corev1.NodeList
		expectedNodes []string
		expectedErr   string
	}{
		{
			name: "All nodes match",
			nodeList: &corev1.NodeList{
				Items: []corev1.Node{
					{ObjectMeta: metav1.ObjectMeta{Name: "node-1", Labels: nodeLabels}},
					{ObjectMeta: metav1.ObjectMeta{Name: "node-2", Labels: nodeLabels}},
					{ObjectMeta: metav1.ObjectMeta{Name: "node-3", Labels: nodeLabels}},
				},
			},
			expectedNodes: []string{"node-1", "node-2", "node-3"},
		},
		{
			name: "Some nodes match",
			nodeList: &corev1.NodeList{
				Items: []corev1.Node{
					{ObjectMeta: metav1.ObjectMeta{Name: "node-1", Labels: nodeLabels}},
					{ObjectMeta: metav1.ObjectMeta{Name: "node-2"}},
					{ObjectMeta: metav1.ObjectMeta{Name: "node-3", Labels: nodeLabels}},
				},
			},
			expectedNodes: []string{"node-1", "node-3"},
		},
		{
			name: "No nodes match",
			nodeList: &corev1.NodeList{
				Items: []corev1.Node{
					{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}},
					{ObjectMeta: metav1.ObjectMeta{Name: "node-2"}},
					{ObjectMeta: metav1.ObjectMeta{Name: "node-3"}},
				},
			},
			expectedErr: "none of the nodes match label selector: MatchLabels: map[node-x:z], MatchExpressions: []",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			nodes, err := findMatchingNodes(test.nodeList, nodeSelector)
			if test.expectedErr != "" {
				require.EqualError(t, err, test.expectedErr)
				assert.Nil(t, nodes)
				return
			}

			require.NoError(t, err)
			require.Len(t, nodes, len(test.expectedNodes))
			for _, expected := range test.expectedNodes {
				assert.True(t, slices.ContainsFunc(nodes, func(actual corev1.Node) bool {
					return actual.Name == expected
				}))
			}
		})
	}
}

func TestIsKubernetesUpgraded(t *testing.T) {
	const kubernetesVersion = "v1.30.3+k3s1"

	tests := []struct {
		name            string
		nodes           []corev1.Node
		expectedUpgrade bool
	}{
		{
			name: "All nodes upgraded",
			nodes: []corev1.Node{
				{
					Status: corev1.NodeStatus{
						Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}},
						NodeInfo:   corev1.NodeSystemInfo{KubeletVersion: "v1.30.3+k3s1"}},
				},
				{
					Status: corev1.NodeStatus{
						Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}},
						NodeInfo:   corev1.NodeSystemInfo{KubeletVersion: "v1.30.3+k3s1"}},
				},
				{
					Status: corev1.NodeStatus{
						Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}},
						NodeInfo:   corev1.NodeSystemInfo{KubeletVersion: "v1.30.3+k3s1"}},
				},
			},
			expectedUpgrade: true,
		},
		{
			name: "Unschedulable node",
			nodes: []corev1.Node{
				{
					Spec: corev1.NodeSpec{Unschedulable: true},
					Status: corev1.NodeStatus{
						Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}},
						NodeInfo:   corev1.NodeSystemInfo{KubeletVersion: "v1.30.3+k3s1"}},
				},
				{
					Status: corev1.NodeStatus{
						Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}},
						NodeInfo:   corev1.NodeSystemInfo{KubeletVersion: "v1.30.3+k3s1"}},
				},
				{
					Status: corev1.NodeStatus{
						Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}},
						NodeInfo:   corev1.NodeSystemInfo{KubeletVersion: "v1.30.3+k3s1"}},
				},
			},
			expectedUpgrade: false,
		},
		{
			name: "Not ready node",
			nodes: []corev1.Node{
				{
					Status: corev1.NodeStatus{
						Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionFalse}},
						NodeInfo:   corev1.NodeSystemInfo{KubeletVersion: "v1.30.3+k3s1"}},
				},
				{
					Status: corev1.NodeStatus{
						Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}},
						NodeInfo:   corev1.NodeSystemInfo{KubeletVersion: "v1.30.3+k3s1"}},
				},
				{
					Status: corev1.NodeStatus{
						Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}},
						NodeInfo:   corev1.NodeSystemInfo{KubeletVersion: "v1.30.3+k3s1"}},
				},
			},
			expectedUpgrade: false,
		},
		{
			name: "Node on older Kubernetes version",
			nodes: []corev1.Node{
				{
					Status: corev1.NodeStatus{
						Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}},
						NodeInfo:   corev1.NodeSystemInfo{KubeletVersion: "v1.28.12+k3s1"}},
				},
				{
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
			expectedUpgrade: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expectedUpgrade, isKubernetesUpgraded(test.nodes, kubernetesVersion))
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

func TestTargetKubernetesVersion(t *testing.T) {
	kubernetes := &lifecyclev1alpha1.Kubernetes{
		K3S: lifecyclev1alpha1.KubernetesDistribution{
			Version: "v1.30.3+k3s1",
		},
		RKE2: lifecyclev1alpha1.KubernetesDistribution{
			Version: "v1.30.3+rke2r1",
		},
	}

	tests := []struct {
		name            string
		nodes           *corev1.NodeList
		expectedVersion string
		expectedError   string
	}{
		{
			name:          "Empty node list",
			nodes:         &corev1.NodeList{},
			expectedError: "unable to determine current kubernetes version due to empty node list",
		},
		{
			name: "Unsupported Kubernetes version",
			nodes: &corev1.NodeList{
				Items: []corev1.Node{{Status: corev1.NodeStatus{NodeInfo: corev1.NodeSystemInfo{KubeletVersion: "v1.30.3"}}}},
			},
			expectedError: "upgrading from kubernetes version v1.30.3 is not supported",
		},
		{
			name: "Target k3s version",
			nodes: &corev1.NodeList{
				Items: []corev1.Node{{Status: corev1.NodeStatus{NodeInfo: corev1.NodeSystemInfo{KubeletVersion: "v1.28.12+k3s1"}}}},
			},
			expectedVersion: "v1.30.3+k3s1",
		},
		{
			name: "Target RKE2 version",
			nodes: &corev1.NodeList{
				Items: []corev1.Node{{Status: corev1.NodeStatus{NodeInfo: corev1.NodeSystemInfo{KubeletVersion: "v1.28.12+rke2r1"}}}},
			},
			expectedVersion: "v1.30.3+rke2r1",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			version, err := targetKubernetesVersion(test.nodes, kubernetes)
			if test.expectedError != "" {
				require.Error(t, err)
				assert.EqualError(t, err, test.expectedError)
				assert.Empty(t, version)
			} else {
				require.NoError(t, err)
				assert.Equal(t, test.expectedVersion, version)
			}
		})
	}
}
