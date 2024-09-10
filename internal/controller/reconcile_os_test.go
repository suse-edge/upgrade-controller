package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	lifecyclev1alpha1 "github.com/suse-edge/upgrade-controller/api/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestValidateOSArch(t *testing.T) {
	validArchs := []lifecyclev1alpha1.Arch{
		"x86_64",
		"aarch64",
	}

	nodes := &corev1.NodeList{
		Items: []corev1.Node{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "node1"},
				Status:     corev1.NodeStatus{NodeInfo: corev1.NodeSystemInfo{Architecture: "arm64"}},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "node2"},
				Status:     corev1.NodeStatus{NodeInfo: corev1.NodeSystemInfo{Architecture: "aarch64"}},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "node3"},
				Status:     corev1.NodeStatus{NodeInfo: corev1.NodeSystemInfo{Architecture: "amd64"}},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "node4"},
				Status:     corev1.NodeStatus{NodeInfo: corev1.NodeSystemInfo{Architecture: "x86_64"}},
			},
		}}

	assert.NoError(t, validateOSArch(nodes, validArchs))
}

func TestValidateOSArch_InvalidArch(t *testing.T) {
	archs := []lifecyclev1alpha1.Arch{
		"x86_64",
		"risc-v",
	}

	assert.PanicsWithValue(t, "unknown arch: risc-v", func() {
		_ = validateOSArch(nil, archs)
	})
}

func TestValidateOSArch_UnsupportedNode(t *testing.T) {
	validArchs := []lifecyclev1alpha1.Arch{
		"x86_64",
		"aarch64",
	}

	nodes := &corev1.NodeList{
		Items: []corev1.Node{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "node1"},
				Status:     corev1.NodeStatus{NodeInfo: corev1.NodeSystemInfo{Architecture: "arm64"}},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "node2"},
				Status:     corev1.NodeStatus{NodeInfo: corev1.NodeSystemInfo{Architecture: "aarch64"}},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "node3"},
				Status:     corev1.NodeStatus{NodeInfo: corev1.NodeSystemInfo{Architecture: "amd64"}},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "node4"},
				Status:     corev1.NodeStatus{NodeInfo: corev1.NodeSystemInfo{Architecture: "x86_64"}},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "node5"},
				Status:     corev1.NodeStatus{NodeInfo: corev1.NodeSystemInfo{Architecture: "risc-v"}},
			},
		}}

	assert.EqualError(t, validateOSArch(nodes, validArchs),
		"unsupported arch 'risc-v' for 'node5' node. Supported archs: [x86_64 aarch64]")
}
