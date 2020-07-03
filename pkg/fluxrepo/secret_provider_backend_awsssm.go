package fluxrepo

import (
	"bytes"
	"fmt"

	"github.com/aws/aws-sdk-go/service/ssm"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/variantdev/vals/pkg/awsclicompat"
	yaml "gopkg.in/yaml.v3"
)

type AWSOptions struct {
	Region  string
	Profile string
}

type AWSSSMBackend struct {
	Path    string
	Version string

	AWSOptions
}

func (s *AWSSSMBackend) FormatRef(ns, name, dataKey string) string {
	return fmt.Sprintf("ref+awsssm://%s?mode=singleparam&version=%s#/%s/%s/%s", s.Path, s.Version, ns, name, dataKey)
}

func (s *AWSSSMBackend) Save(sec map[string]map[string]Secret) error {
	m := ssm.New(awsclicompat.NewSession(s.Region, s.Profile))

	var buf bytes.Buffer

	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)

	if err := enc.Encode(sec); err != nil {
		return err
	}

	secretString := buf.String()

	path := s.Path

	if path[0] != '/' {
		path = "/" + path
	}

	createdParam, putErr := m.PutParameter(&ssm.PutParameterInput{
		Description: aws.String("flux-repo secret"),
		Name:        aws.String(path),
		Tags: []*ssm.Tag{{
			Key:   aws.String("flux-repo"),
			Value: aws.String("managed"),
		}},
		Type:  aws.String(ssm.ParameterTypeSecureString),
		Value: aws.String(secretString),
	})
	if putErr != nil {
		switch putErr.(type) {
		case *ssm.ParameterAlreadyExists:
			createdParam, putErr = m.PutParameter(&ssm.PutParameterInput{
				Description: aws.String("flux-repo secret"),
				Name:        aws.String(path),
				Overwrite:   aws.Bool(true),
				Type:        aws.String(ssm.ParameterTypeSecureString),
				Value:       aws.String(secretString),
			})

			if putErr != nil {
				return fmt.Errorf("overwriting ssm parameter: %w", putErr)
			}
		default:
			return fmt.Errorf("putting ssm parameter: %w", putErr)
		}
	}

	s.Version = fmt.Sprintf("%d", *createdParam.Version)

	return nil
}
