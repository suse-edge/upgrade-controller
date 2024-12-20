package upgrade

import (
	"fmt"
	"strings"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ContainerImage struct {
	Name    string
	Version string
}

func (image ContainerImage) String() string {
	return fmt.Sprintf("%s:%s", image.Name, image.Version)
}

func ReleaseManifestInstallJob(releaseManifest, kubectl ContainerImage, serviceAccount, namespace string, labels map[string]string) (*batchv1.Job, error) {
	if releaseManifest.Name == "" {
		return nil, fmt.Errorf("release manifest image is empty")
	} else if releaseManifest.Version == "" {
		return nil, fmt.Errorf("release manifest version is empty")
	}

	workloadName := fmt.Sprintf("apply-release-manifest-%s", strings.ReplaceAll(releaseManifest.Version, ".", "-"))
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
			Name:      workloadName,
			Namespace: namespace,
			Labels:    labels,
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
							Image:        releaseManifest.String(),
							Command:      []string{"cp", "release_manifest.yaml", releaseManifestPath},
							VolumeMounts: []corev1.VolumeMount{volumeMount},
						},
					},
					Containers: []corev1.Container{
						{
							Name:         workloadName,
							Image:        kubectl.String(),
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
