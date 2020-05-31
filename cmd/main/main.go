package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	"github.com/variantdev/vals"
	"github.com/variantdev/vals/pkg/awsclicompat"
	"gopkg.in/yaml.v3"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

type Secret map[string]string

type awsSecretProvider struct {
	path string

	secrets map[string]map[string]Secret

	versionID string
}

func (s *awsSecretProvider) Add(ns string, name string, dataKey string, dataValue string) {
	nsSec, ok := s.secrets[ns]
	if !ok {
		nsSec = map[string]Secret{}

		s.secrets[ns] = nsSec
	}

	sec, ok := nsSec[name]
	if !ok {
		sec = Secret{}

		nsSec[name] = sec
	}

	sec[dataKey] = dataValue
}

func (s *awsSecretProvider) GetRef(ns string, name string, dataKey string) (string, error) {
	_, ok := s.secrets[ns][name]
	if !ok {
		return "", fmt.Errorf("BUG: no secret registered for %s/%s/%s", ns, name, dataKey)
	}

	return fmt.Sprintf("ref+awssecrets://%s?version_id=%s#/%s/%s/%s", s.path, s.versionID, ns, name, dataKey), nil
}

func (s *awsSecretProvider) Save() error {
	m := secretsmanager.New(awsclicompat.NewSession(""))

	var buf bytes.Buffer

	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)

	if err := enc.Encode(s.secrets); err != nil {
		return err
	}

	secretString := buf.String()

	createdSecret, createErr := m.CreateSecret(&secretsmanager.CreateSecretInput{
		Description:  aws.String("flux-repo secret"),
		Name:         aws.String(s.path),
		SecretString: aws.String(secretString),
		Tags: []*secretsmanager.Tag{{
			Key:   aws.String("flux-repo"),
			Value: aws.String("managed"),
		}},
	})

	if createErr != nil {
		if _, exists := createErr.(*secretsmanager.ResourceExistsException); exists {
			r, putErr := m.PutSecretValue(&secretsmanager.PutSecretValueInput{
				SecretId:     aws.String(s.path),
				SecretString: aws.String(secretString),
			})

			s.versionID = *r.VersionId

			return putErr
		}

		return createErr
	} else {
		s.versionID = *createdSecret.VersionId
	}

	return nil
}

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
		writeCmd := flag.NewFlagSet(CmdWrite, flag.ExitOnError)
		p := writeCmd.String("p", "", "Path to the secret stored in the secrets store")
		f := writeCmd.String("f", "-", "YAML/JSON file or directory to be decoded")
		d := writeCmd.String("o", "", "The output directory")
		_ = writeCmd.String("r", "", "The config repo to be updated with the sanitized manifests")

		if len(os.Args) < 3 {
			flag.Usage()
			return
		}

		if err := writeCmd.Parse(os.Args[2:]); err != nil {
			fatal("%v", err)
		}

		var dir string

		if d == nil || *d == "" {
			tmpfile, err := ioutil.TempFile("", "flux-repo-")
			if err != nil {
				fatal("%v", err)
			}
			tmpfile.Close()
			os.Remove(tmpfile.Name())

			dir = tmpfile.Name()
		} else {
			dir = *d
		}

		if err := os.MkdirAll(dir, 0755); err != nil {
			fatal("%v", err)
		}

		yamlFiles, err := ReadYAMLFiles(*f)
		if err != nil {
			fatal("%v", err)
		}

		if p == nil || *p == "" {
			fatal("missing -p")
		}

		secrets := &awsSecretProvider{
			path:    *p,
			secrets: map[string]map[string]Secret{},
			// This is populated after secrets.Save()
			versionID: "",
		}

		for _, nodes := range yamlFiles {
			var res []yaml.Node
			for _, node := range nodes {
				// Schedule all the secrets to be stored in the secrets store
				n, err := sanitizeSecrets(secrets, node, true)
				if err != nil {
					fatal("%v", err)
				}
				res = append(res, *n)
			}
		}

		// Actually store all the scheduled secrets and obtain the version id
		if err := secrets.Save(); err != nil {
			fatal("%v", err)
		}

		for path, nodes := range yamlFiles {
			var res []yaml.Node
			for _, node := range nodes {
				// Replace secrets' data with references
				n, err := sanitizeSecrets(secrets, node, false)
				if err != nil {
					fatal("%v", err)
				}
				res = append(res, *n)
			}

			var relpath string
			if path != *f {
				relpath = strings.TrimPrefix(path, *f)
			} else {
				relpath = path
			}

			dest := filepath.Join(dir, relpath)

			(func() {
				if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
					fatal("%v", err)
				}

				f, err := os.Create(dest)
				if err != nil {
					fatal("%v", err)
				}
				defer f.Close()

				//var w io.Writer

				// buf := &bytes.Buffer{}
				// w = buf
				w := bufio.NewWriter(f)

				encoder := yaml.NewEncoder(w)
				encoder.SetIndent(2)

				for i, node := range res {
					if err := encoder.Encode(&node); err != nil {
						fatal("%v", err)
					}

					if i != len(res)-1 {
						if _, err := w.Write([]byte("---\n")); err != nil {
							fatal("%v", err)
						}
					}
				}

				encoder.Close()

				if err := w.Flush(); err != nil {
					fatal("%v", err)
				}

				fmt.Printf("Wrote %s to %s\n", path, dest)
			})()
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

		yamlFiles, err := ReadYAMLFiles(f)
		if err != nil {
			fatal("%v", err)
		}

		runtime, err := vals.New(vals.Options{})
		if err != nil {
			fatal("%v", err)
		}

		for _, nodes := range yamlFiles {
			var res []yaml.Node
			for _, node := range nodes {
				n, err := restoreSecrets(runtime, node)
				if err != nil {
					fatal("%v", err)
				}
				res = append(res, *n)
			}

			for i, node := range res {
				buf := &bytes.Buffer{}
				encoder := yaml.NewEncoder(buf)
				encoder.SetIndent(2)

				if err := encoder.Encode(&node); err != nil {
					fatal("%v", err)
				}

				print(buf.String())

				if i != len(res)-1 {
					fmt.Println("---")
				}
			}
		}
	default:
		flag.Usage()
	}
}

func restoreSecrets(r *vals.Runtime, node yaml.Node) (*yaml.Node, error) {
	if node.Kind != yaml.DocumentNode {
		return nil, fmt.Errorf("unexpected kind of node: expected %d, got %d", yaml.DocumentNode, node.Kind)
	}

	var res yaml.Node
	res = node

	var kk yaml.Node
	var vv yaml.Node
	var ii int

	isSecret := false
	mappings := node.Content[0].Content
	for i := 0; i < len(mappings); i += 2 {
		j := i + 1
		k := mappings[i]
		v := mappings[j]

		if k.Value == "kind" && v.Value == "Secret" {
			isSecret = true
		}

		if isSecret && k.Value == "stringData" {
			ii = i
			kk = *k
			vv = *v
		}
	}

	if isSecret {
		stringDataNodeValue := vv

		stringDataMappingNodes := stringDataNodeValue.Content
		stringDataNodeValue.Content = make([]*yaml.Node, len(stringDataNodeValue.Content))
		for i := 0; i < len(stringDataMappingNodes); i += 2 {
			valNode := stringDataMappingNodes[i+1]

			refValue := valNode.Value
			if !strings.HasPrefix(refValue, "ref+") {
				return nil, fmt.Errorf("unexpected secret data value: it must start with ref+ to be restored: got %q", refValue)
			}

			dataKey := "sec"
			dec, err := r.Eval(map[string]interface{}{dataKey: refValue})
			if err != nil {
				return nil, err
			}

			origValue := dec[dataKey].(string)

			valNode.Value = origValue

			keyNode := stringDataMappingNodes[i]

			stringDataNodeValue.Content[i] = keyNode
			stringDataNodeValue.Content[i+1] = valNode
		}

		res.Content[0].Content[ii] = &kk
		res.Content[0].Content[ii+1] = &stringDataNodeValue
	}

	return &res, nil
}

func sanitizeSecrets(secrets *awsSecretProvider, node yaml.Node, add bool) (*yaml.Node, error) {
	if node.Kind != yaml.DocumentNode {
		return nil, fmt.Errorf("unexpected kind of node: expected %d, got %d", yaml.DocumentNode, node.Kind)
	}

	var res yaml.Node
	res = node

	var kk yaml.Node
	var vv yaml.Node
	var ii int

	var ns, name string

	isSecret := false
	mappings := node.Content[0].Content
	for i := 0; i < len(mappings); i += 2 {
		j := i + 1
		k := mappings[i]
		v := mappings[j]

		if k.Value == "kind" && v.Value == "Secret" {
			isSecret = true
		}

		if k.Value == "metadata" {
			for mi := 0; mi < len(v.Content); mi += 2 {
				mj := mi + 1
				mk := v.Content[mi]
				mv := v.Content[mj]

				switch mk.Value {
				case "namespace":
					ns = mv.Value
				case "name":
					name = mv.Value
				}
			}
		}

		if isSecret && k.Value == "data" {
			ii = i
			kk = *k
			vv = *v
		}
	}

	if isSecret {
		if name == "" {
			panic("BUG: No metadata.name found for secret")
		}

		stringDataNodeValue := vv

		stringDataMappingNodes := stringDataNodeValue.Content
		stringDataNodeValue.Content = make([]*yaml.Node, len(stringDataNodeValue.Content))
		for i := 0; i < len(stringDataMappingNodes); i += 2 {
			keyNode := stringDataMappingNodes[i]
			valNode := stringDataMappingNodes[i+1]

			origValue := valNode.Value
			if strings.HasPrefix(origValue, "ref+") {
				return nil, fmt.Errorf("unexpected secret data value: it must NOT start with ref+ to be sanitized")
			}

			if add {
				secrets.Add(ns, name, keyNode.Value, origValue)
			} else {
				refValue, err := secrets.GetRef(ns, name, keyNode.Value)
				if err != nil {
					return nil, err
				}
				valNode.Value = refValue
			}

			stringDataNodeValue.Content[i] = keyNode
			stringDataNodeValue.Content[i+1] = valNode
		}

		if !add {
			kk.Value = "stringData"
		}

		res.Content[0].Content[ii] = &kk
		res.Content[0].Content[ii+1] = &stringDataNodeValue
	}

	return &res, nil
}

func ReadYAMLFiles(f string) (map[string][]yaml.Node, error) {
	var files []string

	stat, err := os.Stat(f)
	if err != nil {
		return nil, err
	}

	if stat.IsDir() {
		matches, err := filepath.Glob(filepath.Join(f, "*"))
		if err != nil {
			return nil, err
		}

		files = append(files, matches...)
	} else {
		files = append(files, f)
	}

	res := map[string][]yaml.Node{}

	for _, f := range files {
		nodes := []yaml.Node{}

		var reader io.Reader
		if f == "-" {
			reader = os.Stdin
		} else if f != "" {
			fp, err := os.Open(f)
			if err != nil {
				return nil, err
			}
			reader = fp
			defer fp.Close()
		} else {
			return nil, fmt.Errorf("Nothing to eval: No file specified")
		}

		buf := bufio.NewReader(reader)
		decoder := yaml.NewDecoder(buf)
		for {
			node := yaml.Node{}
			if err := decoder.Decode(&node); err != nil {
				if err != io.EOF {
					return nil, err
				}
				break
			}
			nodes = append(nodes, node)
		}

		res[f] = nodes
	}

	return res, nil
}
