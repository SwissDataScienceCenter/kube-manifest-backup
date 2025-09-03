# kube-manifest-backup

`kube-manifest-backup` is a Go-based tool designed to back up Kubernetes YAML manifest files to cloud storage backends supported by rclone (S3, Azure Blob Storage, Google Cloud Storage, and many others).

## Features

- Backup Kubernetes YAML manifest files to multiple cloud storage backends via rclone (S3, Azure Blob Storage, Google Cloud Storage, and 40+ others).
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
| -------------------------------- | ---------------------------------- | ---------------------- | -------------------------------------------------- |
| `--use-private-gpg-key`          | `KMB_USE_PRIVATE_GPG_KEY`          | `false`                | use a private GPG key to encrypt backups           |
| `--private-key-secret-name`      | `KMB_PRIVATE_KEY_SECRET_NAME`      | `sops-gpg`             | name of the secret containing the private key      |
| `--private-key-secret-namespace` | `KMB_PRIVATE_KEY_SECRET_NAMESPACE` | `flux-system`          | namespace of the secret containing the private key |
| `--private-key-secret-key`       | `KMB_PRIVATE_KEY_SECRET_KEY`       | `sops.asc`             | key in the secret containing the private key       |
| `--backup-schedule`              | `KMB_BACKUP_SCHEDULE`              | `1/1 * * * *`          | cron schedule for backups                          |
| `--local-backup-dir`             | `KMB_LOCAL_BACKUP_DIR`             | `backups`              | local directory to store backups                   |
| `--run-once`                     | `KMB_RUN_ONCE`                     | `false`                | run a single backup and exit                       |
| `--in-cluster`                   | `KMB_IN_CLUSTER`                   | `false`                | use in-cluster config                              |
| `--backup-resources-yaml-file`   | `KMB_BACKUP_RESOURCES_YAML_FILE`   | `resources.yaml`       | YAML file containing resources to backup           |
| `--config-file`                  | `KMB_CONFIG_FILE`                  | `config.json`          | Storage backend configuration file                 |
| `--bucket-name`                  | `KMB_BUCKET_NAME`                  | `kube-manifest-backup` | Storage bucket/container name                      |
| `--backup-dir`                   | `KMB_BACKUP_DIR`                   | `target-directory`     | Storage backup directory                           |

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

### Storage Backend Configuration

Configure your storage backend connection using [Rclone config parameters](https://rclone.org/). The tool supports all rclone backends by specifying the appropriate `type` and configuration parameters.

#### S3 Example:

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

#### Azure Blob Storage Example:

```json
{
  "type": "azureblob",
  "account": "your-storage-account",
  "key": "your-storage-key",
  "chunk_size": "5Mi",
  "copy_cutoff": "4.656Gi",
  "use_multipart_uploads": "false",
  "use_already_exists": "false",
  "list_chunk": "1000"
}
```

#### Google Cloud Storage Example:

```json
{
  "type": "googlecloudstorage",
  "service_account_file": "/path/to/service-account.json",
  "project_number": "your-project-number",
  "chunk_size": "5Mi",
  "copy_cutoff": "4.656Gi"
}
```

See the [Rclone documentation](https://rclone.org/#providers) for configuration parameters for other supported backends including Dropbox, OneDrive, BackBlaze B2, and many others.

## Contributing

Contributions are welcome! Please open an issue or submit a pull request.

## License

`kube-manifest-backup` is licensed under the [Apache 2.0 license](https://github.com/SwissDataScienceCenter/kube-manifest-backup/blob/main/LICENSE).
