package fluxrepo

import (
	"bytes"
	"fmt"
	"path/filepath"

	"github.com/variantdev/vals"
	yaml "gopkg.in/yaml.v3"
)

func Read(path string) error {
	yamlFiles, err := ReadYAMLFiles(path)
	if err != nil {
		return err
	}

	runtime, err := vals.New(vals.Options{})
	if err != nil {
		return err
	}

	var fileIndex int

	for path, nodes := range yamlFiles {
		// For sops backend, the user may have saved the encrypted file under the same directory as the target files
		// If we didn't skip the encrypted file, it is emitted as-is, which breaks e.g. `flux-repo read | kubectl apply -f -`.
		if filepath.Ext(path) == ".enc" {
			continue
		}

		var res []yaml.Node
		for _, node := range nodes {
			n, err := RestoreSecrets(runtime, node)
			if err != nil {
				return err
			}
			res = append(res, *n)
		}

		for i, node := range res {
			buf := &bytes.Buffer{}
			encoder := yaml.NewEncoder(buf)
			encoder.SetIndent(2)

			if err := encoder.Encode(&node); err != nil {
				return err
			}

			print(buf.String())

			if i != len(res)-1 {
				fmt.Println("---")
			}
		}

		if fileIndex != len(yamlFiles)-1 {
			fmt.Println("---")
		}

		fileIndex++
	}

	return nil
}
