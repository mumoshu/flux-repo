package fluxrepo

import (
	"bytes"
	"fmt"
	"github.com/aws/aws-sdk-go/service/s3"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/variantdev/vals/pkg/awsclicompat"
	yaml "gopkg.in/yaml.v3"
)

type S3Backend struct {
	Key     string
	Version string

	AWSOptions
}

func (s *S3Backend) FormatRef(ns, name, dataKey string) string {
	return fmt.Sprintf("ref+s3://%s?version=%s#/%s/%s/%s", s.Key, s.Version, ns, name, dataKey)
}

func (s *S3Backend) Save(sec map[string]map[string]Secret) error {
	m := s3.New(awsclicompat.NewSession(s.Region, s.Profile))

	var buf bytes.Buffer

	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)

	if err := enc.Encode(sec); err != nil {
		return err
	}

	split := strings.SplitN(s.Key, "/", 2)
	bucket := split[0]
	key := split[1]

	putObj, putErr := m.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(buf.Bytes()),
	})
	if putErr != nil {
		return fmt.Errorf("putting s3 object: %w", putErr)
	}

	s.Version = fmt.Sprintf("%s", *putObj.VersionId)

	return nil
}
