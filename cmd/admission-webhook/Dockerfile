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
COPY cmd/admission-webhook/main.go cmd/admission-webhook/main.go
COPY pkg/ pkg/
copy internal/ internal/
COPY Makefile Makefile

# Build
RUN make admission-webhook

FROM quay.io/prometheus/busybox-${OS}-${ARCH}:latest

COPY --from=builder workspace/admission-webhook /bin/admission-webhook

USER nobody

ENTRYPOINT ["/bin/admission-webhook"]
