package upgrade

import (
	"fmt"
	"strings"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ReleaseManifestInstallJob(releaseManifestImage, releaseManifestVersion, kubectlImage, kubectlVersion, serviceAccount, namespace string, annotations map[string]string) (*batchv1.Job, error) {
	if releaseManifestImage == "" {
		return nil, fmt.Errorf("release manifest image is empty")
	} else if releaseManifestVersion == "" {
		return nil, fmt.Errorf("release manifest version is empty")
	}

	releaseManifestVersion = strings.TrimPrefix(releaseManifestVersion, "v")
	workloadName := fmt.Sprintf("apply-release-manifest-%s", strings.ReplaceAll(releaseManifestVersion, ".", "-"))
	releaseManifestImage = fmt.Sprintf("%s:%s", releaseManifestImage, releaseManifestVersion)
	ttl := int32(0)

	volumeMount := corev1.VolumeMount{
		Name:      "release",
		MountPath: "/release",
	}
	releaseManifestPath := "/release/manifest.yaml"

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
					InitContainers: []corev1.Container{
						{
							Name:         fmt.Sprintf("init-%s", workloadName),
							Image:        releaseManifestImage,
							Command:      []string{"cp", "release_manifest.yaml", releaseManifestPath},
							VolumeMounts: []corev1.VolumeMount{volumeMount},
						},
					},
					Containers: []corev1.Container{
						{
							Name:         workloadName,
							Image:        fmt.Sprintf("%s:%s", kubectlImage, kubectlVersion),
							Args:         []string{"apply", "-f", releaseManifestPath},
							VolumeMounts: []corev1.VolumeMount{volumeMount},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: volumeMount.Name,
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
					RestartPolicy:      "OnFailure",
					ServiceAccountName: serviceAccount,
				},
			},
			TTLSecondsAfterFinished: &ttl,
		},
	}, nil
}
