# flux-repo

`flux-repo` is a companion tool for https://github.com/fluxcd/flux that manages GitOps config repositories and secrets.

The notable feature of it is to "transforming any secret contained in the Kubernetes manifests to references, and vice versa".

This is handy when you prefer NOT commiting encrypted secrets into the config repository and instead want to store secrets themselves into a secret manager, still leaving enough information on the commited manifests so that you can see if there are changes in secrets via git diff.

Why you need this?

Flux allows you to generate Kubernetes manifests for secrets on apply via [manifest generation](https://docs.fluxcd.io/en/1.17.1/references/fluxyaml-config-files.html).

But all the examples around that use-case relies on a tool like `sops` that requires you to commit encrypted secrets into a Git repository. Don't get me wrong - That's okay as long as the encrypted KMS key isn't leaked, and it's pretty safe.

But in certain situations like you need additional audit per secret access (not per key access), you can't commit encrypted secrets into a Git repository. An alternative is to store secrets into a sort of secrets manager. If you do it wrong, you can lose track of which secrets needs to be deployed. The only way would then be storing references to secrets in the git repository while storing secrets themselves into a secrets manager, which gives you the best of both worlds.

You won't like to invent it yourself, so here's the one created for you!

## Supported Backends

- AWS SecretsManager
- Vault (kv v2)

Any [vals](https://github.com/variantdev/vals) backend not listed here, like GCP secrets, can be easily ported to this project. Please feel free to submit a feature/pull request if you want this project to suppory additional backends.

## Usage

```hcl
$ flux-repo -h
Manage GitOps config repositories and secrets for Flux CD

Usage:
  flux-repo [command]
Available Commands:
  write		Produces sanitized Kubernetes manifests by extracting secrets data into a secrets store
  read		Reads sanitized Kubernetes manifests and writes raw manifests for apply
```

### write

```
flux-repo write -h
Usage of write:
  -b string
    	The name of secret provider backend to use (default "awssecrets")
  -f string
    	YAML/JSON file or directory to be decoded (default "-")
  -o string
    	The output directory
  -p string
    	Path to the secret stored in the secrets store
  -r string
    	The config repo to be updated with the sanitized manifests
  -vault-address string
    	The address of Vault API server
  -vault-approle-role-id string
    	Vault role_id for "appauth" authentication. Used only when -vault-auth-method is "approle"
  -vault-approle-secret-id string
    	Vault secret_id for "appauth" authentication. Used only when -vault-auth-method is "approle"
  -vault-auth-method string
    	Auth method for Vault. Use "token" or "approle"
  -vault-token-env string
    	The name of envvar to obtain Vault token from (default "VAULT_TOKEN")
  -vault-token-file string
    	The Vault token file for authentication
```

This command:

- Reads secrets data from secrets in manifests under `inputdir`
- Writes secrets data to the secrets store at the path `foo/bar`
- Exports K8s secrets under `outdir`. Secret resources' `data` are replaced with `stringData` whose values are references, not their original secret values.

For each write under the same secrets store path, `flux-repo` creates a new secret version (search for `AWS Secrets Manager Secret Version` for e.g. AWS) rather than a brand-new secret, so that a lot of writes doesn't result in a lot of secrets store secrets and huge cost.

```
$ kustomize build . > inputdir/all.yaml
$ flux-repo write -b awssecrets -p foo/bar -f inputdir -o outdir
$ ls outdir
all.yaml
```

Let's say `inputdir/all.yaml` was like:

```yaml
apiVersion: v1
kind: Secret
metadata:
  namespace: ns1
  name: foo
data:
  foo: Rk9P
  bar: QkFS
```

`outdir/all.yaml` would look like the below, which is safe to be committed into a git repo:

```yaml
apiVersion: v1
kind: Secret
metadata:
  namespace: ns1
  name: foo
stringData:
  foo: ref+awssecrets://foo/bar?version_id=B0FA5329-CD35-489E-A013-F3639346ACB0#/ns1/foo/foo
  bar: ref+awssecrets://foo/bar?version_id=B0FA5329-CD35-489E-A013-F3639346ACB0#/ns1/foo/bar
```

And the AWS Secrets Manager secret `foo/bar` would look like:

```yaml
$ aws secretsmanager get-secret-value --secret-id foo/bar --version-id B0FA5329-CD35-489E-A013-F3639346ACB0
{
    "ARN": "arn:aws:secretsmanager:us-east-2:ACCOUNT_ID:secret:foo/bar-IdH8XY",
    "Name": "foo/bar",
    "VersionId": "B0FA5329-CD35-489E-A013-F3639346ACB0",
    "SecretString": "ns1:\n  foo:\n    bar: QkFS\n    foo: Rk9P\nns2:\n  bar:\n    bar: QkFS\n    foo: Rk9P\n",
    "VersionStages": [
        "AWSCURRENT"
    ],
    "CreatedDate": 1590913222.888
}
```

### read

- Reads secret references from `foo/bar`
- Reads manifests from `outdir`
- Exports K8s resources to stdout. For each secret resource, references in `stringData` are replaced with their original values fetched from the secrets store

```
flux-repo read outdir | kubectl apply -f -
```

Let's say `outdir/all.yaml` was like:

```yaml
apiVersion: v1
kind: Secret
metadata:
  namespace: ns1
  name: foo
stringData:
  foo: ref+awssecrets://foo/bar?version_id=B0FA5329-CD35-489E-A013-F3639346ACB0#/ns1/foo/foo
  bar: ref+awssecrets://foo/bar?version_id=B0FA5329-CD35-489E-A013-F3639346ACB0#/ns1/foo/bar
```

The output would look like:

```yaml
apiVersion: v1
kind: Secret
metadata:
  namespace: ns1
  name: foo
data:
  foo: Rk9P
  bar: QkFS
```

### With fluxd

For use with fluxd, add `flux-repo` binary to your custom fluxd container image, and create `.flux.yaml` in the repository root:

```
version: 1
patchUpdated:
  generators:
  - command: flux-repo read .
```

This will let flux reads secrets references and generate manifests for K8s secrets, which is then consumed by Flux as usual.

### Using Vault backend

`flux-repo`'s Vault backend requires Vault `kv` backend version 2.

So firstly enable the engine and mount it at e.g. the path `foo/bar`:

```console
$ vault secrets enable -version=2 -path foo/bar kv
Success! Enabled the kv secrets engine at: foo/bar/

$ vault secrets list
Path          Type         Accessor              Description
----          ----         --------              -----------
cubbyhole/    cubbyhole    cubbyhole_54b5eee1    per-token private secret storage
foo/bar/      kv           kv_9bbb11af           n/a
identity/     identity     identity_b092c94f     identity store
kv/           kv           kv_385e23c8           n/a
secret/       kv           kv_0e27a405           key/value secret storage
sys/          system       system_4559871c       system endpoints used for control, policy and debugging
```

The path `foo/bar` becomes the prefix of `-p` in `flux-repo write` so that a write would be run like:

```console
$ flux-repo write -p foo/bar/data/baz -b vault -f examples/simple/in -o examples/simple/out/vault
```

Please note that `data` in `foo/bar/data/baz` is required for `kv` backend v2.

You can verify the necessity of `data` in the path by running `vault kv -output-curl-string` like:

```
$ vault kv put -output-curl-string foo/bar/baz somekey=somevalue
curl -X PUT -H "X-Vault-Token: $(vault print token)" -d '{"data":{"somekey":"somevalue"},"options":{}}' http://127.0.0.1:8200/v1/foo/bar/data/baz
```

The previous `flux-repo write` produces YAML files under `examples/simple/out/vault` as specified by the `-o` flag.

Those YAML files would look like below:

```
apiVersion: v1
kind: Secret
metadata:
  namespace: ns1
  name: foo
stringData:
  # printf FOO | base64
  foo: ref+vault://foo/bar/data/baz?version=1#/ns1/foo/foo
  # printf BAR | base64
  bar: ref+vault://foo/bar/data/baz?version=1#/ns1/foo/bar
```

As you can see in the `ref+` urls, secrets' data fields are stored within the secret at `foo/bar/data/baz`.

You can verify the content of secrets' data by runnign `vault kv get`:

```
$ vault kv get -format json -version 10 foo/bar/baz
{
  "request_id": "3d84ad3d-2aa9-8d96-db2f-89a42193f663",
  "lease_id": "",
  "lease_duration": 0,
  "renewable": false,
  "data": {
    "data": {
      "ns1": {
        "foo": {
          "bar": "BAR",
          "foo": "FOO"
        }
      },
      "ns2": {
        "bar": {
          "bar": "BAR",
          "foo": "FOO"
        }
      }
    },
    "metadata": {
      "created_time": "2020-06-01T12:14:41.394053Z",
      "deletion_time": "",
      "destroyed": false,
      "version": 13
    }
  },
  "warnings": null
}
```

Please note that other K8s resources like Deployment, Pod, Job and so on are exported as-is.

To restore the secrets, run `flux-repo read` like:

```
$ flux-repo examples/simple/out/vault
apiVersion: v1
kind: Secret
metadata:
  namespace: ns1
  name: foo
stringData:
  # printf FOO | base64
  foo: FOO
  # printf BAR | base64
  bar: BAR
---
# other files
```
