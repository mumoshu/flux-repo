package fluxrepo

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"

	yaml "gopkg.in/yaml.v3"
)

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
