package option

type MITMServiceOptions struct {
	Enabled         bool   `json:"enabled,omitempty"`
	Insecure        bool   `json:"insecure,omitempty"`
	Certificate     string `json:"certificate,omitempty"`
	CertificatePath string `json:"certificate_path,omitempty"`
	Key             string `json:"key,omitempty"`
	KeyPath         string `json:"key_path,omitempty"`
}
