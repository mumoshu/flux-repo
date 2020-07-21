package fluxrepo

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/variantdev/vals"
	yaml "gopkg.in/yaml.v3"
)

type Secret map[string]string

func RestoreSecrets(r *vals.Runtime, node yaml.Node) (*yaml.Node, error) {
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

		if k.Value == "stringData" {
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

func SanitizeSecrets(secrets *SecretProvider, node yaml.Node, add bool) (*yaml.Node, error) {
	if node.Kind != yaml.DocumentNode {
		return nil, fmt.Errorf("unexpected kind of node: expected %d, got %d", yaml.DocumentNode, node.Kind)
	}

	var res yaml.Node
	res = node

	var kk yaml.Node
	var vv yaml.Node
	var ii int

	var ns, name string

	var hasData, isStringData bool

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

		if k.Value == "data" {
			ii = i
			kk = *k
			vv = *v

			hasData = true
		}

		if k.Value == "stringData" {
			ii = i
			kk = *k
			vv = *v

			isStringData = true
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

			rawOrigValue := valNode.Value

			var origValue string

			if hasData {
				bs, err := ioutil.ReadAll(base64.NewDecoder(base64.StdEncoding, strings.NewReader(rawOrigValue)))
				if err != nil {
					return nil, fmt.Errorf("uenxpected error while decoding base64-encded secret data field: %w", err)
				}

				origValue = string(bs)
			} else if isStringData {
				origValue = rawOrigValue
			} else {
				panic("BUG: unexpected condition: either data or stringData must be detected before reaching here")
			}

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
