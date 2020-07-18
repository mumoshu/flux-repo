package fluxrepo

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/mumoshu/flux-repo/pkg/encrypt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	yaml "gopkg.in/yaml.v3"
)

type WriteInfo struct {
	Dir string
}

func FilterWithSops(sop *encrypt.Sops, outputDir *string, fsPath *string) (*WriteInfo, error) {
	dir, err := fallbackToTempDir(outputDir)
	if err != nil {
		return nil, err
	}

	yamlFiles, err := FindFiles(*fsPath)
	if err != nil {
		return nil, err
	}

	for _, path := range yamlFiles {
		file, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("opening file %s: %w", file, err)
		}

		fileContent, err := ioutil.ReadAll(file)
		if err != nil {
			return nil, fmt.Errorf("reading file %s: %w", path, err)
		}

		dec := yaml.NewDecoder(bytes.NewReader(fileContent))
		type objectMeta struct {
			Kind string `yaml:"kind"`
		}

		var objMeta objectMeta
		if err := dec.Decode(&objMeta); err != nil {
			return nil, fmt.Errorf("decoding yaml file %s: %w", path, err)
		}

		var data []byte

		if objMeta.Kind != "Secret" {
			data = fileContent
		} else {
			enc, err := sop.Data(path, fileContent, strings.TrimPrefix(filepath.Ext(path), "."))
			if err != nil {
				return nil, fmt.Errorf("encryptiong %s: %w", path, err)
			}

			data = enc
		}

		var relpath string
		if path != *fsPath {
			relpath = strings.TrimPrefix(path, *fsPath)
		} else {
			relpath = path
		}

		dest := filepath.Join(dir, relpath)

		destDir := filepath.Dir(dest)
		if err := os.MkdirAll(destDir, 0755); err != nil {
			return nil, fmt.Errorf("creating directory %s: %w", destDir, err)
		}

		if err := ioutil.WriteFile(dest, data, 0644); err != nil {
			return nil, fmt.Errorf("writing file %s: %w", dest, err)
		}

		if err != nil {
			return nil, fmt.Errorf("writing file to %s: %w", dest, err)
		}
	}

	return &WriteInfo{Dir: dir}, nil
}

func fallbackToTempDir(outputDir *string) (string, error) {
	var dir string

	if outputDir == nil || *outputDir == "" {
		tmpfile, err := ioutil.TempFile("", "flux-repo-")
		if err != nil {
			return "", err
		}
		tmpfile.Close()
		os.Remove(tmpfile.Name())

		dir = tmpfile.Name()
	} else {
		dir = *outputDir
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}

	return dir, nil
}

func Write(backend SecretProviderBackend, outputDir *string, fsPath *string) (*WriteInfo, error) {
	dir, err := fallbackToTempDir(outputDir)
	if err != nil {
		return nil, err
	}

	yamlFiles, err := ReadYAMLFiles(*fsPath)
	if err != nil {
		return nil, err
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
				return nil, err
			}
			res = append(res, *n)
		}
	}

	// Actually store all the scheduled secrets and obtain the version id
	if err := secrets.Save(); err != nil {
		return nil, err
	}

	for path, nodes := range yamlFiles {
		var res []yaml.Node
		for _, node := range nodes {
			// Replace secrets' data with references
			n, err := SanitizeSecrets(secrets, node, false)
			if err != nil {
				return nil, err
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
			return nil, err
		}
	}

	return &WriteInfo{Dir: dir}, nil
}
