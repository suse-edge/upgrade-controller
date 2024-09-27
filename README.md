# Upgrade Controller

A Kubernetes controller capable of performing infrastructure platform upgrades consisting of:
* Operating System (SL Micro)
* Kubernetes (k3s & RKE2)
* Additional components (Rancher, Elemental, NeuVector, etc.)

## Requirements

### System Upgrade Controller

The Upgrade Controller utilizes the [System Upgrade Controller](https://github.com/rancher/system-upgrade-controller)
to perform OS and Kubernetes upgrades. Ensure that it is installed on the cluster e.g. via the respective
[Helm chart](https://github.com/rancher/charts/tree/release-v2.9/charts/system-upgrade-controller/104.0.0%2Bup0.7.0).

OS upgrades consist of both package updates within the same OS version (e.g. SL Micro 6.0) and migration to later versions
(e.g. SL Micro 6.0 -> SL Micro 6.1).

Kubernetes upgrades are generally advised to never skip a minor version (e.g. 1.28 -> 1.30). Proceed with caution
as such scenarios are not prevented by the Upgrade Controller and may lead to unexpected behaviour.

### Helm Controller

Additional components installed on the cluster via Helm charts are being upgraded by the
[Helm Controller](https://github.com/k3s-io/helm-controller). Both k3s and RKE2 clusters have this controller
built-in. It is enabled by default and users of the Upgrade Controller should ensure that it is not manually
disabled via the respective CLI argument or config file parameter.

## Workflow

The Upgrade Controller reconciles **UpgradePlan** resources. These follow a very simple definition:

```yaml
apiVersion: lifecycle.suse.com/v1alpha1
kind: UpgradePlan
metadata:
  name: upgrade-plan-3-1-0
  namespace: upgrade-controller-system
spec:
  releaseVersion: 3.1.0
```

While there are few additional fields which can influence how the different upgrades are performed,
none of those are mandatory.

The most important field is `releaseVersion` which maps to a **ReleaseManifest** resource.
This resource contains the information necessary for all the different components (OS, Kubernetes, etc.).

The Upgrade Controller will look for such **ReleaseManifest** on the cluster. If it is present, it will be used.
If not, it will be pulled from a container image source (which is configurable).

Once the release manifest is fetched, the Upgrade Controller will start the execution of the plan.

It will go through the following stages:

**1. OS upgrade**

OS upgrades will be executed on the control plane first, and on the worker nodes second.
Each upgrade is happening one node at a time, and each node will be handled individually i.e.
one node may have some installed packages on newer or older versions than the others,
however the upgrade process will bring them to the same state.

**2. Kubernetes upgrade**

Similarly to the OS upgrades, Kubernetes upgrades follow the control plane first approach
and all nodes are also being upgraded one at a time.

**3. Additional components upgrade**

Currently, all additional components are installed via Helm charts. Some of those have dependencies (e.g. CRD charts)
or add-ons (e.g. Rancher dashboard extensions). The upgrades will follow the order of the component list within the release manifest.
Each Helm component upgrade may receive additional values coming from either the release manifest or the upgrade plan, or both.

Once the upgrade plan goes through all of these stages, it is considered finished. Refer to its status for the information about each step.
