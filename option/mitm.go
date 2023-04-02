package option

type MITMServiceOptions struct {
	Enabled         bool             `json:"enabled,omitempty"`
	Insecure        bool             `json:"insecure,omitempty"`
	Certificate     string           `json:"certificate,omitempty"`
	CertificatePath string           `json:"certificate_path,omitempty"`
	Key             string           `json:"key,omitempty"`
	KeyPath         string           `json:"key_path,omitempty"`
	HTTP            *MITMHTTPOptions `json:"http,omitempty"`
}

type MITMHTTPOptions struct {
	Enabled        bool     `json:"enabled,omitempty"`
	URLRewritePath []string `json:"url_rewrite_path,omitempty"`
}
