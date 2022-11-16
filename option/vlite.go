package option

type VLiteOutboundOptions struct {
	DialerOptions
	ServerOptions
	Password                    string `json:"password,omitempty"`
	ScramblePacket              bool   `json:"scramble_packet,omitempty"`
	EnableFEC                   bool   `json:"enable_fec,omitempty"`
	EnableStabilization         bool   `json:"enable_stabilization,omitempty"`
	EnableRenegotiation         bool   `json:"enable_renegotiation,omitempty"`
	HandshakeMaskingPaddingSize int    `json:"handshake_masking_padding_size,omitempty"`
}
