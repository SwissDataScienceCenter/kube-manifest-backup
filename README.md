# kube-manifest-backup

`kube-manifest-backup` is a Go-based tool designed to back up Kubernetes YAML manifest files to an S3 bucket.

## Features

- Backup Kubernetes YAML manifest files to an S3 bucket.
- Cron-based scheduling.
- Support for backing up secrets, encrypted with an in-cluster GPG key.
- Export Prometheus metrics

## Installation

### Helm

```bash
helm repo add renku https://swissdatasciencecenter.github.io/helm-charts/
helm install kube-manifest-backup renku/kube-manifest-backup -f your-values-file.yaml
```

### Binary

Binary releases are available on the [releases page](https://github.com/SwissDataScienceCenter/kube-manifest-backup/releases).

## Configuration and Usage

If installed using the Helm chart, `kube-manifest-backup` can be configured using the values specified in the `values.yaml` file.

Otherwise, `kube-manifest-backup` can be configured using command-line flags or environment variables:

The following command-line flags and environment variables can be used to configure the tool:

| CLI Flag                         | Environment Variable               | Default Value          | Description                                        |
|----------------------------------|------------------------------------|------------------------|----------------------------------------------------|
| `--use-private-gpg-key`          | `KMB_USE_PRIVATE_GPG_KEY`          | `false`                | use a private GPG key to encrypt backups           |
| `--private-key-secret-name`      | `KMB_PRIVATE_KEY_SECRET_NAME`      | `sops-gpg`             | name of the secret containing the private key      |
| `--private-key-secret-namespace` | `KMB_PRIVATE_KEY_SECRET_NAMESPACE` | `flux-system`          | namespace of the secret containing the private key |
| `--private-key-secret-key`       | `KMB_PRIVATE_KEY_SECRET_KEY`       | `sops.asc`             | key in the secret containing the private key       |
| `--backup-schedule`              | `KMB_BACKUP_SCHEDULE`              | `1/1 * * * *`          | cron schedule for backups                          |
| `--local-backup-dir`             | `KMB_LOCAL_BACKUP_DIR`             | `backups`              | local directory to store backups                   |
| `--run-once`                     | `KMB_RUN_ONCE`                     | `false`                | run a single backup and exit                       |
| `--in-cluster`                   | `KMB_IN_CLUSTER`                   | `false`                | use in-cluster config                              |
| `--backup-resources-yaml-file`   | `KMB_BACKUP_RESOURCES_YAML_FILE`   | `resources.yaml`       | YAML file containing resources to backup           |
| `--s3-config-file`               | `KMB_S3_CONFIG_FILE`               | `s3-config.json`       | S3 configuration file                              |
| `--s3-bucket-name`               | `KMB_S3_BUCKET_NAME`               | `kube-manifest-backup` | S3 bucket name                                     |
| `--s3-backup-dir`                | `KMB_S3_BACKUP_DIR`                | `target-directory`     | S3 backup directory                                |

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

`kube-manifest-backup` is licensed under the [Apache 2.0 license](https://github.com/SwissDataScienceCenter/kube-manifest-backup/blob/main/LICENSE).
