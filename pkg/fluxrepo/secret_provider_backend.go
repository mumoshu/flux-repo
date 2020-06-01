package fluxrepo

type SecretProviderBackend interface {
	FormatRef(ns, name, dataKey string) string
	Save(map[string]map[string]Secret) error
}

