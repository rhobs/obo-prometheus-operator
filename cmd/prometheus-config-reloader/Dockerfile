ARG ARCH="amd64"
ARG OS="linux"
FROM quay.io/prometheus/golang-builder:main as builder
WORKDIR /workspace
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
# RUN go mod download

# Copy the go source
COPY cmd/prometheus-config-reloader/main.go cmd/prometheus-config-reloader/main.go
COPY pkg/ pkg/
copy internal/ internal/
COPY Makefile Makefile

# Build
RUN make prometheus-config-reloader

FROM quay.io/prometheus/busybox-${OS}-${ARCH}:latest

COPY --from-builder workspace/prometheus-config-reloader /bin/prometheus-config-reloader

USER nobody

ENTRYPOINT ["/bin/prometheus-config-reloader"]
