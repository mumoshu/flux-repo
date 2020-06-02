ARG FLUX_VERSION

FROM fluxcd/flux:${FLUX_VERSION}

LABEL maintainer="Yusuke KUOKA <https://github.com/mumoshu/flux-repo/issues>" \
      org.opencontainers.image.title="flux-repo" \
      org.opencontainers.image.description="Enterprise-grade repository and secrets management for Flux CD" \
      org.opencontainers.image.url="https://github.com/mumoshu/flux-repo" \
      org.opencontainers.image.source="git@github.com:mumoshu/flux-repo" \
      org.opencontainers.image.vendor="mumoshu" \
      org.label-schema.schema-version="1.0" \
      org.label-schema.name="flux-repo" \
      org.label-schema.description="Enterprise-grade repository and secrets management for Flux CD" \
      org.label-schema.url="https://github.com/mumoshu/flux-repo" \
      org.label-schema.vcs-url="git@github.com:mumoshu/flux-repo" \
      org.label-schema.vendor="mumoshu"

COPY ./flux-repo /usr/local/bin/

ENV PATH=/bin:/usr/bin:/usr/local/bin:/usr/lib/kubeyaml

ENTRYPOINT [ "/sbin/tini", "--", "fluxd" ]
