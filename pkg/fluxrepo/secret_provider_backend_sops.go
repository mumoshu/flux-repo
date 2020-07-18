package fluxrepo

import (
	"bytes"
	"fmt"
	"github.com/mumoshu/flux-repo/pkg/encrypt"
	yaml "gopkg.in/yaml.v3"
	"io/ioutil"
	"path/filepath"
	"strings"
)

type SOPSBackend struct {
	KMSKeyARN         string
	EncryptionContext string
	FilePath          string

	AWSOptions
}

func (s *SOPSBackend) FormatRef(ns, name, dataKey string) string {
	return fmt.Sprintf("ref+sops://%s#/%s/%s/%s", s.FilePath, ns, name, dataKey)
}

func (s *SOPSBackend) Save(sec map[string]map[string]Secret) error {
	var buf bytes.Buffer

	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)

	if err := enc.Encode(sec); err != nil {
		return err
	}

	sop := &encrypt.Sops{
		KMS:               s.KMSKeyARN,
		EncryptionContext: s.EncryptionContext,
		AWSProfile:        s.AWSOptions.Profile,
	}

	encryptedData, encryptionErr := sop.Data(s.FilePath, buf.Bytes(), "yaml")
	if encryptionErr != nil {
		return fmt.Errorf("encryptiong secrets to %s: %w", s.FilePath, encryptionErr)
	}

	if err := ioutil.WriteFile(s.FilePath, encryptedData, 0644); err != nil {
		return fmt.Errorf("writing file to %s: %w", s.FilePath, err)
	}

	return nil
}

func (s *SOPSBackend) Validate() error {
	if s.KMSKeyARN == "" {
		return fmt.Errorf("-aws-kms-key-arn must be provided when using sops backend")
	}

	if !strings.HasPrefix(s.KMSKeyARN, "arn:aws:kms:") {
		return fmt.Errorf("validating `-aws-kms-key-arn %q`: it must start with \"arn:aws:kms:\"", s.KMSKeyARN)
	}

	if ext := filepath.Ext(s.FilePath); ext != ".enc" {
		return fmt.Errorf("validating `-p %q`: it must end with .enc when using sops backend", s.FilePath)
	}

	return nil
}
