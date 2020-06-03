.PHONY: build
build:
	go fmt ./...
	goimports -d .
	goimports -w .
	go build -o flux-repo ./cmd/main

.PHONY: image
image:
	docker build --build-arg FLUX_VERSION=1.19.0 -t example/flux-repo:dev .

.PHONY: test-publish
test-publish:
	FLUX_VERSION=1.19.0 goreleaser --snapshot --skip-publish --rm-dist
