package release

type Release struct {
	APIVersion     float64 `yaml:"apiVersion"`
	ReleaseVersion string  `yaml:"releaseVersion"`
	Components     struct {
		Kubernetes struct {
			K3S struct {
				Version string `yaml:"version"`
			} `yaml:"k3s"`
			RKE2 struct {
				Version string `yaml:"version"`
			} `yaml:"rke2"`
		} `yaml:"kubernetes"`
	} `yaml:"components"`
}
