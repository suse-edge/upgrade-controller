package upgrade

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReleaseManifestInstallJob(t *testing.T) {
	releaseManifest := ContainerImage{
		Name:    "registry.suse.com/edge/release-manifest",
		Version: "3.1.0",
	}
	kubectl := ContainerImage{
		Name:    "registry.suse.com/edge/kubectl",
		Version: "1.30.3",
	}
	serviceAccount := "upgrade-controller-sa"
	namespace := "upgrade-controller-ns"
	labels := map[string]string{
		"lifecycle.suse.com/x": "z",
	}

	job, err := ReleaseManifestInstallJob(releaseManifest, kubectl, serviceAccount, namespace, labels)
	require.NoError(t, err)

	assert.Equal(t, "batch/v1", job.TypeMeta.APIVersion)
	assert.Equal(t, "Job", job.TypeMeta.Kind)

	assert.Equal(t, "apply-release-manifest-3-1-0", job.ObjectMeta.Name)
	assert.Equal(t, "upgrade-controller-ns", job.ObjectMeta.Namespace)
	require.Len(t, job.ObjectMeta.Labels, 1)
	assert.Equal(t, labels, job.ObjectMeta.Labels)

	assert.Equal(t, "apply-release-manifest-3-1-0", job.Spec.Template.ObjectMeta.Name)
	assert.Equal(t, "upgrade-controller-ns", job.Spec.Template.ObjectMeta.Namespace)

	require.Len(t, job.Spec.Template.Spec.InitContainers, 1)

	containerSpec := job.Spec.Template.Spec.InitContainers[0]
	assert.Equal(t, "init-apply-release-manifest-3-1-0", containerSpec.Name)
	assert.Equal(t, "registry.suse.com/edge/release-manifest:3.1.0", containerSpec.Image)
	assert.Equal(t, []string{"cp", "release_manifest.yaml", "/release/manifest.yaml"}, containerSpec.Command)
	assert.Empty(t, containerSpec.Args)

	require.Len(t, containerSpec.VolumeMounts, 1)
	assert.Equal(t, "release", containerSpec.VolumeMounts[0].Name)
	assert.Equal(t, "/release", containerSpec.VolumeMounts[0].MountPath)

	require.Len(t, job.Spec.Template.Spec.Containers, 1)

	containerSpec = job.Spec.Template.Spec.Containers[0]
	assert.Equal(t, "apply-release-manifest-3-1-0", containerSpec.Name)
	assert.Equal(t, "registry.suse.com/edge/kubectl:1.30.3", containerSpec.Image)
	assert.Empty(t, containerSpec.Command)
	assert.Equal(t, []string{"apply", "-f", "/release/manifest.yaml"}, containerSpec.Args)

	require.Len(t, containerSpec.VolumeMounts, 1)
	assert.Equal(t, "release", containerSpec.VolumeMounts[0].Name)
	assert.Equal(t, "/release", containerSpec.VolumeMounts[0].MountPath)

	require.Len(t, job.Spec.Template.Spec.Volumes, 1)
	assert.Equal(t, "release", job.Spec.Template.Spec.Volumes[0].Name)
	assert.NotNil(t, job.Spec.Template.Spec.Volumes[0].EmptyDir)

	assert.EqualValues(t, "OnFailure", job.Spec.Template.Spec.RestartPolicy)
	assert.Equal(t, "upgrade-controller-sa", job.Spec.Template.Spec.ServiceAccountName)

	ttl := int32(0)
	assert.Equal(t, &ttl, job.Spec.TTLSecondsAfterFinished)

	job, err = ReleaseManifestInstallJob(ContainerImage{Version: "3.1.0"}, kubectl, serviceAccount, namespace, labels)
	require.Error(t, err)
	assert.EqualError(t, err, "release manifest image is empty")
	assert.Nil(t, job)

	job, err = ReleaseManifestInstallJob(ContainerImage{Name: "registry.suse.com/edge/release-manifest"}, kubectl, serviceAccount, namespace, labels)
	require.Error(t, err)
	assert.EqualError(t, err, "release manifest version is empty")
	assert.Nil(t, job)
}
