FROM --platform=$BUILDPLATFORM golang:1.22.4-alpine AS build
WORKDIR /src
ENV CGO_ENABLED=0
COPY . .
ARG TARGETPLATFORM

RUN echo "Building for $TARGETPLATFORM" && \
    GOOS=$(echo $TARGETPLATFORM | cut -d '/' -f1) \
    GOARCH=$(echo $TARGETPLATFORM | cut -d '/' -f2) \
    go build -o /out/kube-manifest-backup .

FROM alpine:3.17.2 AS bin
WORKDIR /app
COPY --from=build /out/kube-manifest-backup /usr/local/bin/

ENV KMB_USE_PRIVATE_GPG_KEY="false"
ENV KMB_PRIVATE_KEY_SECRET_NAME="sops-gpg"
ENV KMB_PRIVATE_KEY_SECRET_NAMESPACE="flux-system"
ENV KMB_PRIVATE_KEY_SECRET_KEY="sops.asc"
ENV KMB_BACKUP_SCHEDULE="1/1 * * * *"
ENV KMB_LOCAL_BACKUP_DIR="backups"
ENV KMB_RUN_ONCE="false"
ENV KMB_IN_CLUSTER="false"
ENV KMB_BACKUP_RESOURCES_YAML_FILE="resources.yaml"
ENV KMB_S3_CONFIG_FILE="s3-config.json"
ENV KMB_S3_BUCKET_NAME="kube-manifest-backup"
ENV KMB_S3_BACKUP_DIR="backups"

EXPOSE 2112/tcp

ENTRYPOINT []

CMD /usr/local/bin/kube-manifest-backup
