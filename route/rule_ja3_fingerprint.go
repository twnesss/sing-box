package route

import (
	"strings"

	"github.com/sagernet/sing-box/adapter"
)

var _ RuleItem = (*JA3FingerprintItem)(nil)

type JA3FingerprintItem struct {
	fingerprints   []string
	fingerprintMap map[string]bool
}

func NewJA3FingerprintItem(hashList []string) *JA3FingerprintItem {
	rule := &JA3FingerprintItem{
		fingerprints:   hashList,
		fingerprintMap: make(map[string]bool),
	}
	for _, hash := range hashList {
		rule.fingerprintMap[strings.ToLower(hash)] = true
	}
	return rule
}

func (r *JA3FingerprintItem) Match(metadata *adapter.InboundContext) bool {
	if metadata.JA3Fingerprint == "" {
		return false
	}
	return r.fingerprintMap[metadata.JA3Fingerprint]
}

func (r *JA3FingerprintItem) String() string {
	var description string
	pLen := len(r.fingerprints)
	if pLen == 1 {
		description = "ja3_fingerprint=" + r.fingerprints[0]
	} else {
		description = "ja3_fingerprint=[" + strings.Join(r.fingerprints, " ") + "]"
	}
	return description
}
