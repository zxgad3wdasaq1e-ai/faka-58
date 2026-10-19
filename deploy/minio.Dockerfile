# MinIO 的官方安全版本要求从源码构建；固定官方 release，不依赖旧 Docker Hub 镜像。
# https://github.com/minio/minio/releases/tag/RELEASE.2025-10-15T17-29-55Z
FROM --platform=$BUILDPLATFORM golang:1.26.5-alpine AS build
ARG TARGETOS=linux
ARG TARGETARCH
ARG MINIO_VERSION=RELEASE.2025-10-15T17-29-55Z
ARG MC_VERSION=RELEASE.2025-08-13T08-35-41Z
ENV CGO_ENABLED=0
RUN apk add --no-cache git ca-certificates
RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} go install -trimpath github.com/minio/minio@${MINIO_VERSION} \
    && mkdir /out \
    && (cp /go/bin/*/minio /out/minio 2>/dev/null || cp /go/bin/minio /out/minio)
RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} go install -trimpath github.com/minio/mc@${MC_VERSION} \
    && (cp /go/bin/*/mc /out/mc 2>/dev/null || cp /go/bin/mc /out/mc)
RUN cp /go/pkg/mod/github.com/minio/minio@*/LICENSE /out/MINIO-LICENSE \
    && cp /go/pkg/mod/github.com/minio/mc@*/LICENSE /out/MC-LICENSE

FROM alpine:3.22 AS server
RUN apk add --no-cache ca-certificates tzdata
COPY --from=build /out/minio /usr/local/bin/minio
COPY --from=build /out/MINIO-LICENSE /usr/share/licenses/minio/LICENSE
EXPOSE 9000 9001
ENTRYPOINT ["minio"]

FROM alpine:3.22 AS client
RUN apk add --no-cache ca-certificates
COPY --from=build /out/mc /usr/local/bin/mc
COPY --from=build /out/MC-LICENSE /usr/share/licenses/mc/LICENSE
ENTRYPOINT ["mc"]
