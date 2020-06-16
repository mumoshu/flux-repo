package fluxrepo

import (
	"bytes"
	"fmt"

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

	for _, nodes := range yamlFiles {
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
