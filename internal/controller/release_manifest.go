package controller

import (
	"context"
	"fmt"
	"time"

	lifecyclev1alpha1 "github.com/suse-edge/upgrade-controller/api/v1alpha1"
	"github.com/suse-edge/upgrade-controller/internal/upgrade"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var errReleaseManifestNotFound = fmt.Errorf("release manifest not found")

func (r *UpgradePlanReconciler) retrieveReleaseManifest(ctx context.Context, upgradePlan *lifecyclev1alpha1.UpgradePlan) (*lifecyclev1alpha1.ReleaseManifest, error) {
	manifests := &lifecyclev1alpha1.ReleaseManifestList{}
	listOpts := &client.ListOptions{
		Namespace: upgradePlan.Namespace,
	}
	if err := r.List(ctx, manifests, listOpts); err != nil {
		return nil, fmt.Errorf("listing release manifests in cluster: %w", err)
	}

	for _, manifest := range manifests.Items {
		if manifest.Spec.ReleaseVersion == upgradePlan.Spec.ReleaseVersion {
			return &manifest, nil
		}
	}

	return nil, errReleaseManifestNotFound
}

func (r *UpgradePlanReconciler) createReleaseManifest(ctx context.Context, upgradePlan *lifecyclev1alpha1.UpgradePlan) error {
	annotations := upgrade.PlanIdentifierAnnotations(upgradePlan.Name, upgradePlan.Namespace)
	job, err := upgrade.ReleaseManifestInstallJob(
		r.ReleaseManifestImage,
		upgradePlan.Spec.ReleaseVersion,
		r.KubectlImage,
		r.KubectlVersion,
		r.ServiceAccount,
		upgradePlan.Namespace,
		annotations)
	if err != nil {
		return err
	}

	// Retry the creation since a previously failed job could be in the process of deletion due to TTL
	return retry.OnError(wait.Backoff{Steps: 5, Duration: 500 * time.Millisecond},
		func(err error) bool { return true },
		func() error { return r.createObject(ctx, upgradePlan, job) })
}
