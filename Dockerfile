# syntax=docker/dockerfile:1

# ---- build ----------------------------------------------------------------
# Pinned to BUILDPLATFORM so that multi-arch builds cross-compile natively
# instead of running the whole toolchain under emulation.
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build

ENV CGO_ENABLED=0

WORKDIR /src

# Dependencies first, so editing the source does not invalidate the module
# download layer.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .

ARG VERSION=dev
ARG TARGETOS
ARG TARGETARCH
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} go build \
      -trimpath \
      -ldflags "-s -w -X github.com/jeliasson/dns-propagation-checker/internal/cli.Version=${VERSION}" \
      -o /dnspc ./cmd/dnspc

# ---- runtime --------------------------------------------------------------
# scratch: the image holds one static binary and nothing else. The frontend is
# embedded in the binary and DNS queries are made natively, so there is no
# Node runtime, no dig, and no base distro to keep patched. Nothing here
# speaks TLS, so not even a CA bundle is needed.
FROM scratch AS runtime

COPY --from=build /dnspc /dnspc

# Unprivileged. scratch has no /etc/passwd, so the id has to be numeric.
USER 65534:65534

EXPOSE 3000
ENV DNSPC_ADDR=:3000

# No shell in scratch, so the binary probes itself.
HEALTHCHECK --interval=30s --timeout=3s --start-period=2s \
    CMD ["/dnspc", "healthcheck"]

ENTRYPOINT ["/dnspc"]
CMD ["serve"]
