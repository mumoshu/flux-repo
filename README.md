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

Any [vals](https://github.com/variantdev/vals) backend like HashiCorp Vault or GCP secrets can be easily ported to this project. Please feel free to submit a feature request if you want this project to suppory additional backends.

## Usage

### write

- Reads secrets data from secrets in manifests under `inputdir`
- Writes secrets data to the secrets store at the path `foo/bar`
- Exports K8s secrets under `outdir`. Secret resources' `data` are replaced with `stringData` whose values are references, not their original secret values.

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
