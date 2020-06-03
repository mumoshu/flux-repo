package fluxrepo

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	vault "github.com/hashicorp/vault/api"
)

type VaultBackend struct {
	Address, AuthMethod, TokenFile string
	TokenEnv                       string
	RoleID, SecretID               string

	Path      string
	VersionID string
}

func (s *VaultBackend) FormatRef(ns, name, dataKey string) string {
	return fmt.Sprintf("ref+vault://%s?version=%s#/%s/%s/%s", s.Path, s.VersionID, ns, name, dataKey)
}

func (s *VaultBackend) Save(sec map[string]map[string]Secret) error {
	vc, err := s.createVaultClient()
	if err != nil {
		return err
	}

	//data := map[string]interface{}{}
	//
	//for ns, nsSecrets := range sec {
	//	for name, secrets := range nsSecrets {
	//		k := fmt.Sprintf("%s/%s", ns, name)
	//
	//		var buf bytes.Buffer
	//
	//		enc := yaml.NewEncoder(&buf)
	//		enc.SetIndent(2)
	//
	//		if err := enc.Encode(secrets); err != nil {
	//			return err
	//		}
	//
	//		secretString := buf.String()
	//
	//		data[k] = secretString
	//	}
	//}

	data := map[string]map[string]map[string]string{}

	for k1, v1 := range sec {
		m1 := map[string]map[string]string{}
		for k2, v2 := range v1 {
			m2 := map[string]string{}
			for k3, v3 := range v2 {
				m2[k3] = v3
			}
			m1[k2] = m2
		}
		data[k1] = m1
	}

	// We need the data to be put in the "data" field for Vault kv v2
	wrote, writeErr := vc.Logical().Write(s.Path, map[string]interface{}{"data": data})

	if writeErr != nil {
		return writeErr
	}

	versionJson := wrote.Data["version"].(json.Number)

	s.VersionID = versionJson.String()

	return nil
}

func (p *VaultBackend) createVaultClient() (*vault.Client, error) {
	cfg := vault.DefaultConfig()
	if p.Address != "" {
		cfg.Address = p.Address
	}
	if strings.Contains(p.Address, "127.0.0.1") {
		cfg.ConfigureTLS(&vault.TLSConfig{Insecure: true})
	}
	cli, err := vault.NewClient(cfg)
	if err != nil {
		p.debugf("Vault connections failed")
		return nil, fmt.Errorf("Cannot create Vault Client: %v", err)
	}

	if p.AuthMethod == "token" {
		if p.TokenEnv != "" {
			token := os.Getenv(p.TokenEnv)
			if token == "" {
				return nil, fmt.Errorf("token_env configured to read vault token from envvar %q, but it isn't set", p.TokenEnv)
			}
			cli.SetToken(token)
		}

		if p.TokenFile != "" {
			token, err := p.readTokenFile(p.TokenFile)
			if err != nil {
				return nil, err
			}
			cli.SetToken(token)
		}

		// By default Vault token is set from VAULT_TOKEN env var by NewClient()
		// But if VAULT_TOKEN isn't set, token can be retrieved from ~/.vault-token file
		if cli.Token() == "" {
			homeDir := os.Getenv("HOME")
			if homeDir != "" {
				token, _ := p.readTokenFile(filepath.Join(homeDir, ".vault-token"))
				if token != "" {
					cli.SetToken(token)
				}
			}
		}
	} else if p.AuthMethod == "approle" {
		if p.RoleID == "" {
			return nil, fmt.Errorf("missing role_id for approle auth")
		}

		if p.SecretID == "" {
			return nil, fmt.Errorf("missing secret_id for approle auth")
		}

		data := map[string]interface{}{
			"role_id":   p.RoleID,
			"secret_id": p.SecretID,
		}

		resp, err := cli.Logical().Write("auth/approle/login", data)
		if err != nil {
			return nil, err
		}

		if resp.Auth == nil {
			return nil, fmt.Errorf("no auth info returned")
		}

		cli.SetToken(resp.Auth.ClientToken)
	}
	return cli, nil
}

func (p *VaultBackend) readTokenFile(path string) (string, error) {
	homeDir := os.Getenv("HOME")
	if homeDir != "" {
		buff, err := ioutil.ReadFile(filepath.Join(homeDir, path))
		if err != nil {
			return "", err
		}
		return string(buff), nil
	}
	return "", nil
}

func (p *VaultBackend) debugf(msg string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, msg+"\n", args...)
}
