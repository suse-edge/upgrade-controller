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
		OperatingSystem OperatingSystem `yaml:"operatingSystem"`
	} `yaml:"components"`
}

type OperatingSystem struct {
	Version        string   `yaml:"version"`
	ZypperID       string   `yaml:"zypperID"`
	CPEScheme      string   `yaml:"cpeScheme"`
	RepoGPGPath    string   `yaml:"repoGPGPath"`
	SupportedArchs []string `yaml:"supportedArchs"`
}
