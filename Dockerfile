FROM --platform=$BUILDPLATFORM golang:1.26 AS builder

ARG VERSION
ARG TARGETOS TARGETARCH
ENV PKG github.com/ownkube/kubernetes-events-exporter/internal/buildinfo

ADD . /app
WORKDIR /app
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -ldflags="-s -w -X ${PKG}.Version=${VERSION}" -a -o /main ./cmd/exporter

FROM gcr.io/distroless/static:nonroot
COPY --from=builder --chown=nonroot:nonroot /main /kubernetes-events-exporter

# https://github.com/GoogleContainerTools/distroless/blob/main/base/base.bzl#L8C1-L9C1
USER 65532

ENTRYPOINT ["/kubernetes-events-exporter"]
