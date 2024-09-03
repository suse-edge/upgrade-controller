package upgrade

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReleaseManifestInstallJob(t *testing.T) {
	imageName := "registry.suse.com/edge/release-manifest"
	imageVersion := "3.1.0"
	serviceAccount := "upgrade-controller-sa"
	namespace := "upgrade-controller-ns"
	annotations := map[string]string{
		"lifecycle.suse.com/x": "z",
	}

	job, err := ReleaseManifestInstallJob(imageName, imageVersion, serviceAccount, namespace, annotations)
	require.NoError(t, err)

	assert.Equal(t, "batch/v1", job.TypeMeta.APIVersion)
	assert.Equal(t, "Job", job.TypeMeta.Kind)

	assert.Equal(t, "apply-release-manifest-3-1-0", job.ObjectMeta.Name)
	assert.Equal(t, "upgrade-controller-ns", job.ObjectMeta.Namespace)
	require.Len(t, job.ObjectMeta.Annotations, 1)
	assert.Equal(t, annotations, job.ObjectMeta.Annotations)

	assert.Equal(t, "apply-release-manifest-3-1-0", job.Spec.Template.ObjectMeta.Name)
	assert.Equal(t, "upgrade-controller-ns", job.Spec.Template.ObjectMeta.Namespace)

	require.Len(t, job.Spec.Template.Spec.Containers, 1)

	containerSpec := job.Spec.Template.Spec.Containers[0]
	assert.Equal(t, "apply-release-manifest-3-1-0", containerSpec.Name)
	assert.Equal(t, "registry.suse.com/edge/release-manifest:3.1.0", containerSpec.Image)
	assert.Equal(t, []string{"apply", "-f", "release_manifest.yaml"}, containerSpec.Args)

	assert.EqualValues(t, "OnFailure", job.Spec.Template.Spec.RestartPolicy)
	assert.Equal(t, "upgrade-controller-sa", job.Spec.Template.Spec.ServiceAccountName)

	ttl := int32(0)
	assert.Equal(t, &ttl, job.Spec.TTLSecondsAfterFinished)

	job, err = ReleaseManifestInstallJob("", imageVersion, serviceAccount, namespace, annotations)
	require.Error(t, err)
	assert.EqualError(t, err, "release manifest image is empty")
	assert.Nil(t, job)

	job, err = ReleaseManifestInstallJob(imageName, "", serviceAccount, namespace, annotations)
	require.Error(t, err)
	assert.EqualError(t, err, "release manifest version is empty")
	assert.Nil(t, job)
}
