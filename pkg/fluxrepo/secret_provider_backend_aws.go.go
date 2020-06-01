package fluxrepo

import (
	"bytes"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	"github.com/variantdev/vals/pkg/awsclicompat"
	"gopkg.in/yaml.v3"
)

type AWSSecretsBackend struct {
	Path      string
	VersionID string
}

func (s *AWSSecretsBackend) FormatRef(ns, name, dataKey string) string {
	return fmt.Sprintf("ref+awssecrets://%s?version_id=%s#/%s/%s/%s", s.Path, s.VersionID, ns, name, dataKey)
}

func (s *AWSSecretsBackend) Save(sec map[string]map[string]Secret) error {
	m := secretsmanager.New(awsclicompat.NewSession(""))

	var buf bytes.Buffer

	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)

	if err := enc.Encode(sec); err != nil {
		return err
	}

	secretString := buf.String()

	createdSecret, createErr := m.CreateSecret(&secretsmanager.CreateSecretInput{
		Description:  aws.String("flux-repo secret"),
		Name:         aws.String(s.Path),
		SecretString: aws.String(secretString),
		Tags: []*secretsmanager.Tag{{
			Key:   aws.String("flux-repo"),
			Value: aws.String("managed"),
		}},
	})

	if createErr != nil {
		if _, exists := createErr.(*secretsmanager.ResourceExistsException); exists {
			r, putErr := m.PutSecretValue(&secretsmanager.PutSecretValueInput{
				SecretId:     aws.String(s.Path),
				SecretString: aws.String(secretString),
			})

			s.VersionID = *r.VersionId

			return putErr
		}

		return createErr
	} else {
		s.VersionID = *createdSecret.VersionId
	}

	return nil
}
