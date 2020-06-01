package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/mumoshu/flux-repo/pkg/fluxrepo"
	"os"
)

func flagUsage() {
	text := `Manage GitOps config repositories and secrets for Flux CD

Usage:
  flux-repo [command]
Available Commands:
  write		Produces sanitized Kubernetes manifests by extracting secrets data into a secrets store
  read		Reads sanitized Kubernetes manifests and writes raw manifests for apply

Use "flux-repo [command] --help" for more information about a command
`

	fmt.Fprintf(os.Stderr, "%s\n", text)
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func main() {
	flag.Usage = flagUsage

	CmdWrite := "write"
	CmdRead := "read"

	if len(os.Args) == 1 {
		flag.Usage()
		return
	}

	switch os.Args[1] {
	case CmdWrite:
		var awsBackend fluxrepo.AWSSecretsBackend
		var vaultBackend fluxrepo.VaultBackend

		writeCmd := flag.NewFlagSet(CmdWrite, flag.ExitOnError)
		secretPath := writeCmd.String("p", "", "Path to the secret stored in the secrets store")
		fsPath := writeCmd.String("f", "-", "YAML/JSON file or directory to be decoded")
		outputDir := writeCmd.String("o", "", "The output directory")
		secretBackend := writeCmd.String("b", "awssecrets", "The name of secret provider backend to use")

		writeCmd.StringVar(&vaultBackend.AuthMethod, "vault-auth-method", "", "Auth method for Vault. Use \"token\" or \"approle\"")
		writeCmd.StringVar(&vaultBackend.Address, "vault-address", "", "The address of Vault API server")
		writeCmd.StringVar(&vaultBackend.TokenFile, "vault-token-file", "", "The Vault token file for authentication")
		writeCmd.StringVar(&vaultBackend.TokenEnv, "vault-token-env", "VAULT_TOKEN", "The name of envvar to obtain Vault token from")
		writeCmd.StringVar(&vaultBackend.RoleID, "vault-approle-role-id", "", "Vault role_id for \"appauth\" authentication. Used only when -vault-auth-method is \"approle\" ")
		writeCmd.StringVar(&vaultBackend.SecretID, "vault-approle-secret-id", "", "Vault secret_id for \"appauth\" authentication. Used only when -vault-auth-method is \"approle\" ")

		_ = writeCmd.String("r", "", "The config repo to be updated with the sanitized manifests")

		if len(os.Args) < 3 {
			flag.Usage()
			return
		}

		if err := writeCmd.Parse(os.Args[2:]); err != nil {
			fatal("%v", err)
		}

		backend, err := createBackend(secretBackend, &awsBackend, &vaultBackend, secretPath)
		if err != nil {
			fatal("%v", err)
		}

		if err := fluxrepo.Write(backend, outputDir, fsPath, secretPath); err != nil {
			fatal("%v", err)
		}
	case CmdRead:
		readCmd := flag.NewFlagSet(CmdRead, flag.ExitOnError)

		if len(os.Args) != 3 {
			flag.Usage()
			return
		}

		if err := readCmd.Parse(os.Args[2:]); err != nil {
			fatal("%v", err)
		}

		f := os.Args[2]

		if err := fluxrepo.Read(f); err != nil {
			fatal("%v", err)
		}
	default:
		flag.Usage()
	}
}

func createBackend(backendName *string, awsBackend *fluxrepo.AWSSecretsBackend, vaultBackend *fluxrepo.VaultBackend, secretPath *string) (fluxrepo.SecretProviderBackend, error) {

	if secretPath == nil || *secretPath == "" {
		return nil, errors.New("missing secret path")
	}

	var backend fluxrepo.SecretProviderBackend

	if backendName == nil || *backendName == "awssecrets" {
		awsBackend.Path = *secretPath

		backend = awsBackend
	} else if *backendName == "vault" {
		vaultBackend.Path = *secretPath

		backend = vaultBackend
	} else {
		return nil, fmt.Errorf("unsupported secret provider backend: %v", *backendName)
	}

	return backend, nil
}
