package fluxrepo

import "fmt"

type SecretProvider struct {
	Secrets map[string]map[string]Secret

	backend SecretProviderBackend
}

func (s *SecretProvider) Add(ns string, name string, dataKey string, dataValue string) {
	nsSec, ok := s.Secrets[ns]
	if !ok {
		nsSec = map[string]Secret{}

		s.Secrets[ns] = nsSec
	}

	sec, ok := nsSec[name]
	if !ok {
		sec = Secret{}

		nsSec[name] = sec
	}

	sec[dataKey] = dataValue
}

func (s *SecretProvider) GetRef(ns string, name string, dataKey string) (string, error) {
	_, ok := s.Secrets[ns][name]
	if !ok {
		return "", fmt.Errorf("BUG: no secret registered for %s/%s/%s", ns, name, dataKey)
	}

	return s.backend.FormatRef(ns, name, dataKey), nil
}

func (s *SecretProvider) Save() error {
	return s.backend.Save(s.Secrets)
}

