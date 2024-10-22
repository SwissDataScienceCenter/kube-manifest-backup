# kube-manifest-backup

kube-manifest-backup is a Go-based tool designed to back up Kubernetes YAML manifest files to an S3 bucket.

## Features

- Backup Kubernetes YAML manifest files to an S3 bucket.
- Cron-based scheduling.
- Support for backing up secrets, encrypted with an in-cluster GPG key.
- Export Prometheus metrics

## Installation

## Usage

## Configuration

### `resources.yaml`

Specify the Kubernetes resources you want to back up. Example:

```yaml
resources:
  - namespaces: ["renku"]
    group: ""
    version: "v1"
    resource: "persistentvolumeclaims"
    secret: false
  - namespaces: [""]
    group: ""
    version: "v1"
    resource: "persistentvolumes"
    secret: false
  - namespaces: ["renku"]
    group: "amalthea.dev"
    version: "v1alpha1"
    resource: "jupyterservers"
    secret: false
  - namespaces: ["renku"]
    group: ""
    version: "v1"
    resource: "secrets"
    secret: true
```

### s3-config.json

Configure the S3 connection using [Rclone config parameters](https://rclone.org/s3/#standard-options). Example:

```json
{
    "type": "s3",
    "provider": "Other",
    "access_key_id": "******",
    "secret_access_key": "******",
    "region": "ZH",
    "endpoint": "https://os.zhdk.cloud.switch.ch",
    "env_auth": "false",
    "chunk_size": "5Mi",
    "copy_cutoff": "4.656Gi",
    "list_version": "2",
    "force_path_style": "true",
    "list_url_encode": "false",
    "use_multipart_uploads": "false",
    "use_already_exists": "false",
    "list_chunk": "1000"
}
```

## Contributing
Contributions are welcome! Please open an issue or submit a pull request.

## License