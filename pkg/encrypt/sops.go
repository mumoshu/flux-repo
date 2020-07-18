package encrypt

import (
	"fmt"
	"go.mozilla.org/sops"
	"go.mozilla.org/sops/aes"
	"go.mozilla.org/sops/cmd/sops/codes"
	"go.mozilla.org/sops/cmd/sops/common"
	"go.mozilla.org/sops/keys"
	"go.mozilla.org/sops/keyservice"
	"go.mozilla.org/sops/kms"
	sopsdotenv "go.mozilla.org/sops/stores/dotenv"
	sopsjson "go.mozilla.org/sops/stores/json"
	sopsyaml "go.mozilla.org/sops/stores/yaml"
	"go.mozilla.org/sops/version"
	"io/ioutil"
	"path/filepath"
)

type Sops struct {
	KMS               string
	EncryptionContext string
	AWSProfile        string
}

// File is a wrapper around Data that reads a local cleartext
// file and returns its encrypted data in an []byte
func (sp *Sops) File(path, format string) (cleartext []byte, err error) {
	// Read the file into an []byte
	cleatextFile, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("Failed to read %q: %v", path, err)
	}
	return sp.Data(path, cleatextFile, format)
}

// Data is a helper that takes cleartext data and a format string,
// encrypts the data and returns its encrypted data in an []byte.
// The format string can be `json`, `yaml`, `dotenv` or `binary`.
// If the format string is empty, binary format is assumed.
func (sp *Sops) Data(path string, data []byte, format string) (cleartext []byte, err error) {
	// Initialize a Sops JSON store
	var inputStore sops.Store
	switch format {
	case "json":
		inputStore = &sopsjson.Store{}
	case "yaml":
		inputStore = &sopsyaml.Store{}
	case "dotenv":
		inputStore = &sopsdotenv.Store{}
	default:
		inputStore = &sopsjson.BinaryStore{}
	}
	// Load SOPS file and access the data key
	branches, err := inputStore.LoadPlainFile(data)
	if err != nil {
		return nil, err
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("getting absolute path of %s: %w", path, err)
	}

	var keyGroup sops.KeyGroup

	{
		var kmsKeys []keys.MasterKey
		kmsEncryptionContext := kms.ParseKMSContext(sp.EncryptionContext)
		if sp.EncryptionContext != "" && kmsEncryptionContext == nil {
			return nil, common.NewExitError("Invalid KMS encryption context format", codes.ErrorInvalidKMSEncryptionContextFormat)
		}
		if sp.KMS != "" {
			for _, k := range kms.MasterKeysFromArnString(sp.KMS, kmsEncryptionContext, sp.AWSProfile) {
				kmsKeys = append(kmsKeys, k)
			}
		}

		for _, k := range kmsKeys {
			keyGroup = append(keyGroup, k)
		}
	}

	tree := sops.Tree{
		Branches: branches,
		Metadata: sops.Metadata{
			KeyGroups:         []sops.KeyGroup{keyGroup},
			UnencryptedSuffix: "__unenc",
			// This is set to non-empty when and only when you need opt-in for encryption
			// In other words, you must omit this if you wanna encrypt everything in the data
			EncryptedSuffix:   "",
			Version:           version.Version,
			ShamirThreshold:   0,
		},
		FilePath: absPath,
	}

	var keyServices []keyservice.KeyServiceClient

	keyServices = append(keyServices, keyservice.NewLocalClient())

	dataKey, errs := tree.GenerateDataKeyWithKeyServices(keyServices)
	if len(errs) > 0 {
		err = fmt.Errorf("Could not generate data key: %s", errs)
		return nil, err
	}

	err = common.EncryptTree(common.EncryptTreeOpts{
		DataKey: dataKey,
		Tree:    &tree,
		Cipher:  aes.NewCipher(),
	})
	if err != nil {
		return nil, err
	}

	outputStore := &sopsyaml.Store{}

	encryptedFile, err := outputStore.EmitEncryptedFile(tree)
	if err != nil {
		return nil, common.NewExitError(fmt.Sprintf("Could not marshal tree: %s", err), codes.ErrorDumpingTree)
	}

	return encryptedFile, nil
}
