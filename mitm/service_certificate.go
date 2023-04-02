package mitm

import (
	"crypto/tls"
	"os"

	E "github.com/sagernet/sing/common/exceptions"

	"github.com/fsnotify/fsnotify"
)

func (s *Service) startWatcher() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	if s.certificatePath != "" {
		err = watcher.Add(s.certificatePath)
		if err != nil {
			return err
		}
	}
	if s.keyPath != "" {
		err = watcher.Add(s.keyPath)
		if err != nil {
			return err
		}
	}
	s.watcher = watcher
	go s.loopUpdate()
	return nil
}

func (s *Service) loopUpdate() {
	for {
		select {
		case event, ok := <-s.watcher.Events:
			if !ok {
				return
			}
			if event.Op&fsnotify.Write != fsnotify.Write {
				continue
			}
			err := s.reloadKeyPair()
			if err != nil {
				s.logger.Error(E.Cause(err, "reload TLS key pair"))
			}
		case err, ok := <-s.watcher.Errors:
			if !ok {
				return
			}
			s.logger.Error(E.Cause(err, "fsnotify error"))
		}
	}
}

func (s *Service) reloadKeyPair() error {
	if s.certificatePath != "" {
		certificate, err := os.ReadFile(s.certificatePath)
		if err != nil {
			return E.Cause(err, "reload certificate from ", s.certificatePath)
		}
		s.certificate = certificate
	}
	if s.keyPath != "" {
		key, err := os.ReadFile(s.keyPath)
		if err != nil {
			return E.Cause(err, "reload key from ", s.keyPath)
		}
		s.key = key
	}
	keyPair, err := tls.X509KeyPair(s.certificate, s.key)
	if err != nil {
		return E.Cause(err, "reload key pair")
	}
	s.tlsCertificate = &keyPair
	s.logger.Info("reloaded TLS certificate")
	return nil
}
