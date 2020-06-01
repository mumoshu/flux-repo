package fluxrepo

import (
	"bufio"
	"fmt"
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

func Write(backend SecretProviderBackend, outputDir *string, fsPath *string, secretPath *string) error {
	var dir string

	if outputDir == nil || *outputDir == "" {
		tmpfile, err := ioutil.TempFile("", "flux-repo-")
		if err != nil {
			return err
		}
		tmpfile.Close()
		os.Remove(tmpfile.Name())

		dir = tmpfile.Name()
	} else {
		dir = *outputDir
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	yamlFiles, err := ReadYAMLFiles(*fsPath)
	if err != nil {
		return err
	}

	secrets := &SecretProvider{
		backend: backend,
		Secrets: map[string]map[string]Secret{},
	}

	for _, nodes := range yamlFiles {
		var res []yaml.Node
		for _, node := range nodes {
			// Schedule all the secrets to be stored in the secrets store
			n, err := SanitizeSecrets(secrets, node, true)
			if err != nil {
				return err
			}
			res = append(res, *n)
		}
	}

	// Actually store all the scheduled secrets and obtain the version id
	if err := secrets.Save(); err != nil {
		return err
	}

	for path, nodes := range yamlFiles {
		var res []yaml.Node
		for _, node := range nodes {
			// Replace secrets' data with references
			n, err := SanitizeSecrets(secrets, node, false)
			if err != nil {
				return err
			}
			res = append(res, *n)
		}

		var relpath string
		if path != *fsPath {
			relpath = strings.TrimPrefix(path, *fsPath)
		} else {
			relpath = path
		}

		dest := filepath.Join(dir, relpath)

		err := (func() error {
			if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
				return err
			}

			f, err := os.Create(dest)
			if err != nil {
				return err
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
					return err
				}

				if i != len(res)-1 {
					if _, err := w.Write([]byte("---\n")); err != nil {
						return err
					}
				}
			}

			encoder.Close()

			if err := w.Flush(); err != nil {
				return err
			}

			fmt.Printf("Wrote %s to %s\n", path, dest)

			return nil
		})()

		if err != nil {
			return err
		}
	}

	return nil
}
