package tls

import (
	"crypto/tls"

	E "github.com/sagernet/sing/common/exceptions"
	aTLS "github.com/sagernet/sing/common/tls"
)

type (
	Config                 = aTLS.Config
	ConfigCompat           = aTLS.ConfigCompat
	ServerConfig           = aTLS.ServerConfig
	ServerConfigCompat     = aTLS.ServerConfigCompat
	WithSessionIDGenerator = aTLS.WithSessionIDGenerator
	Conn                   = aTLS.Conn

	STDConfig       = tls.Config
	STDConn         = tls.Conn
	ConnectionState = tls.ConnectionState
)

func ParseTLSVersion(version string) (uint16, error) {
	switch version {
	case "1.0":
		return tls.VersionTLS10, nil
	case "1.1":
		return tls.VersionTLS11, nil
	case "1.2":
		return tls.VersionTLS12, nil
	case "1.3":
		return tls.VersionTLS13, nil
	default:
		return 0, E.New("unknown tls version:", version)
	}
}

func ConfigFromClientHello(clientHello *tls.ClientHelloInfo) *tls.Config {
	minVersion := clientHello.SupportedVersions[0]
	maxVersion := minVersion
	for _, version := range clientHello.SupportedVersions {
		if version > maxVersion {
			maxVersion = version
		}
		if version < minVersion {
			minVersion = version
		}
	}
	return &tls.Config{
		CipherSuites:     clientHello.CipherSuites,
		NextProtos:       clientHello.SupportedProtos,
		ServerName:       clientHello.ServerName,
		MinVersion:       minVersion,
		MaxVersion:       maxVersion,
		CurvePreferences: clientHello.SupportedCurves,
	}
}
