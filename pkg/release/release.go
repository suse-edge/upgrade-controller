package release

type Release struct {
	APIVersion     float64    `yaml:"apiVersion"`
	ReleaseVersion string     `yaml:"releaseVersion"`
	Components     Components `yaml:"components"`
}

type Components struct {
	Kubernetes             Kubernetes      `yaml:"kubernetes"`
	OperatingSystem        OperatingSystem `yaml:"operatingSystem"`
	Rancher                HelmChart       `yaml:"rancher"`
	Longhorn               HelmChart       `yaml:"longhorn"`
	MetalLB                HelmChart       `yaml:"metallb"`
	CDI                    HelmChart       `yaml:"cdi"`
	KubeVirt               KubeVirt        `yaml:"kubevirt"`
	NeuVector              NeuVector       `yaml:"neuvector"`
	EndpointCopierOperator HelmChart       `yaml:"endpointCopierOperator"`
	Elemental              Elemental       `yaml:"elemental"`
	SRIOV                  SRIOV           `yaml:"sriov"`
	Akri                   Akri            `yaml:"akri"`
	Metal3                 HelmChart       `yaml:"metal3"`
}

type Kubernetes struct {
	K3S  KubernetesDistribution `yaml:"k3s"`
	RKE2 KubernetesDistribution `yaml:"rke2"`
}

type KubernetesDistribution struct {
	Version string `yaml:"version"`
}

type OperatingSystem struct {
	Version        string   `yaml:"version"`
	ZypperID       string   `yaml:"zypperID"`
	CPEScheme      string   `yaml:"cpeScheme"`
	RepoGPGPath    string   `yaml:"repoGPGPath"`
	SupportedArchs []string `yaml:"supportedArchs"`
	PrettyName     string   `yaml:"prettyName"`
}

type HelmChart struct {
	ReleaseName string `yaml:"releaseName"`
	Name        string `yaml:"chart"`
	Repository  string `yaml:"repository,omitempty"`
	Version     string `yaml:"version"`
}

type NeuVector struct {
	CRD       HelmChart `yaml:"crd"`
	NeuVector HelmChart `yaml:"neuvector"`
}

type Elemental struct {
	CRD      HelmChart `yaml:"crd"`
	Operator HelmChart `yaml:"operator"`
}

type SRIOV struct {
	CRD             HelmChart `yaml:"crd"`
	NetworkOperator HelmChart `yaml:"networkOperator"`
}

type KubeVirt struct {
	KubeVirt           HelmChart `yaml:"kubevirt"`
	DashboardExtension HelmChart `yaml:"dashboardExtension"`
}

type Akri struct {
	Akri               HelmChart `yaml:"akri"`
	DashboardExtension HelmChart `yaml:"dashboardExtension"`
}
