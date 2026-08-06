FROM registry.suse.com/bci/bci-nano:16.0@sha256:b3cef3f3e085f6dc8e2fb7071cab19a30ec98b902e4428236cc50bf87feef3cb

ARG VERSION=dev
ARG REVISION=unknown
ARG CREATED=unknown

LABEL org.opencontainers.image.title="TDNS" \
      org.opencontainers.image.description="DNS over TLS forwarder with caching and runtime reconfiguration" \
      org.opencontainers.image.source="https://github.com/jfardello/tdns" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${REVISION}" \
      org.opencontainers.image.created="${CREATED}" \
      org.opencontainers.image.licenses="MIT"

COPY --chown=65532:65532 tdns /usr/local/bin/tdns

EXPOSE 8053/udp 8443/tcp

USER 65532:65532
ENTRYPOINT ["/usr/local/bin/tdns"]
