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

Writes secrets data to the AWS Secrets Manager secret at the path `foo/bar/flux-repo-TIME-ID/secrets.json` and modifies K8s secrets resources to NOT include data/stringData and have annotations with the references to written secrets data:

```
kustomize build . | flux-repo write github.com/myorg/myconfig/sub --backend awssec://foo/bar path/to/dir
```

Reads `secrets.json` from the AWS Secrets Manager secret at ``foo/bar/flux-repo-TIME-ID/secrets.json`, and transforms K8s secrets resources annotations to data/stringData, writes the resulting manifests to stdout:

```
flux-repo read . | kubectl apply -f -
```

For use with fluxd, add `flux-repo` binary to your custom fluxd container image, and create `.flux.yaml` in the repository root:

```
version: 1
patchUpdated:
  generators:
  - command: flux-repo read .
```

This will let flux reads secrets references and generate manifests for K8s secrets, which is then consumed by Flux as usual.
