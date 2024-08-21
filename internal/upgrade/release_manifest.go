package upgrade

import (
	"fmt"
	"strings"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ReleaseManifestInstallJob(image, version, namespace string, annotations map[string]string) *batchv1.Job {
	version = strings.TrimPrefix(version, "v")
	workloadName := fmt.Sprintf("apply-release-manifest-%s", strings.ReplaceAll(version, ".", "-"))
	image = fmt.Sprintf("%s:%s", image, version)
	ttl := int32(0)

	return &batchv1.Job{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "batch/v1",
			Kind:       "Job",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        workloadName,
			Namespace:   namespace,
			Annotations: annotations,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      workloadName,
					Namespace: namespace,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  workloadName,
							Image: image,
							Args:  []string{"apply", "-f", "release_manifest.yaml"},
						},
					},
					RestartPolicy:      "OnFailure",
					ServiceAccountName: "upgrade-controller-controller-manager",
				},
			},
			TTLSecondsAfterFinished: &ttl,
		},
	}
}
