# Pi Controller - Developer Guidelines

## Packaging & Release Rules

- **Client File Distribution**: ONLY the `docker` folder (and specific binaries defined in `config.json`) is packaged and uploaded in a release. Therefore, **any files (e.g. configuration files like YAMLs) destined for the end device MUST be placed inside the `docker` folder.** 
- If a `docker-compose.yml` mounts a file to be sent to the client, ensure that the file exists within the `docker` directory itself and is mounted with a relative `./` path, rather than reaching into `pkg/` or other root folders which are NOT shipped to the client device.

## Understanding `config.json`

The `config.json` file dictates how a release is packaged, validated, and eventually applied on the client by the `runner` binary. It is crucial to understand the distinct purpose of each field:

### 1. `update_directories`
- **Purpose**: Defines which directories should be packaged in the release tarball (e.g., `"docker"`). 
- **Runner Behavior**: The `runner` binary will **only create** these directories on the client if they don't exist. It will **NOT** automatically copy files inside these directories from the extracted update folder to the live application directory.

### 2. `update_files`
- **Purpose**: A strict, exhaustive list of every single file that must be included in the release and updated on the client.
- **Validation**: The `scripts/validate_release.go` script verifies that every file present in the generated `.tar.gz` exactly matches an entry in either `update_directories` or `update_files`. If an extra file is found in the tarball that isn't explicitly listed here, the build will fail.
- **Runner Behavior**: The `runner` binary iterates **only over this list** when applying an update. If a file (like `docker/loki-config.yaml`) is packaged in the tarball but NOT explicitly listed in `update_files`, the `runner` will completely ignore it, fail to move it into the live application directory, and delete it during cleanup.
- **Rule**: If you add a new file to the `docker/` directory that the client needs, you **MUST** explicitly append its path (e.g. `"docker/new-file.yaml"`) to the `update_files` array.

### 3. `root_files`
- **Purpose**: Used by the `runner` during the cleanup phase of an update. Any file listed in `update_files` that is NOT in `root_files` is treated as an ephemeral or updatable asset and will be deleted from the live directory right before the new version is moved into place.
