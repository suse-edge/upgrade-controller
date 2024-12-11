package upgrade

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
)

// ContainsContainerImages validates that a given map of "container: image" references
// exist in a slice of corev1.Containers.
//
// Returns 'true' only if all the "container: image" references from the 'contains' map
// are present in the corev1.Containers slice.
//
// If 'strict' is true, will require for the corev1.Container.Image to be exactly the same
// as the image string defined in the 'contains' map.
//
// If 'strict' is false, will require for the corev1.Container.Image to contain the image string defined
// in the 'contains' map. Useful for use-cases where the image registry may change
// based on the environment (e.g. private registry).
func ContainsContainerImages(containers []corev1.Container, contains map[string]string, strict bool) bool {
	foundContainers := 0
	for _, container := range containers {
		image, ok := contains[container.Name]
		if !ok {
			// Skip containers that are not in the 'contains' map.
			continue
		}
		foundContainers++

		if strict && container.Image != image {
			return false
		}

		if !strict && !strings.Contains(container.Image, image) {
			return false
		}
	}

	return foundContainers == len(contains)
}
