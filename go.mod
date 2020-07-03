module github.com/mumoshu/flux-repo

go 1.13

require (
	github.com/aws/aws-sdk-go v1.29.34
	github.com/hashicorp/vault/api v1.0.5-0.20190909201928-35325e2c3262
	github.com/variantdev/vals v0.7.0
	gopkg.in/yaml.v3 v3.0.0-20200506231410-2ff61e1afc86
)

//replace github.com/variantdev/vals => ../vals
