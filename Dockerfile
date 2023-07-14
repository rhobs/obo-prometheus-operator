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
COPY cmd/operator/main.go cmd/operator/main.go
COPY pkg/ pkg/
copy internal/ internal/
COPY Makefile Makefile

# Build
RUN make operator

FROM quay.io/prometheus/busybox-${OS}-${ARCH}:latest

COPY --from=builder workspace/operator /bin/operator

# On busybox 'nobody' has uid `65534'
USER 65534

ENTRYPOINT ["/bin/operator"]
