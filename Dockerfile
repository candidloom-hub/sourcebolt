# RPC-only image: static binary, CA certificates, nothing else.
# GoReleaser (dockers_v2) puts the prebuilt binary at $TARGETPLATFORM/sourcebolt
# in the build context. For a manual build: see docs in scripts/ or run
# `goreleaser release --snapshot --clean --skip=publish,sign,sbom`.
FROM --platform=$BUILDPLATFORM alpine:3.24.3@sha256:be2656061b536c1883c4034790e21d0a09c5aaaf51968b89368a3c0371ea2c9a AS certs
RUN apk add --no-cache ca-certificates

FROM scratch
ARG TARGETPLATFORM
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY $TARGETPLATFORM/sourcebolt /sourcebolt
# Profiles and the 0600 key file (no OS keyring in a container) live here;
# mount a volume at /config so credentials survive container restarts.
ENV NOTEBOOKLM_CONFIG_DIR=/config
VOLUME ["/config"]
USER 65532:65532
ENTRYPOINT ["/sourcebolt"]
CMD ["serve"]
