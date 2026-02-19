# k0sctl
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fk0sproject%2Fk0sctl.svg?type=shield)](https://app.fossa.com/projects/git%2Bgithub.com%2Fk0sproject%2Fk0sctl?ref=badge_shield)


*A command-line bootstrapping and management tool for [k0s zero friction kubernetes](https://k0sproject.io/) clusters.*

- [Installation](#installation)
- [Development status](#development-status)
- [Usage](#usage)
- [Configuration](#configuration-file)

Example output of k0sctl deploying a k0s cluster:

```text
INFO ==> Running phase: Connect to hosts
INFO ==> Running phase: Detect host operating systems
INFO [ssh] 10.0.0.1:22: is running Ubuntu 20.10
INFO [ssh] 10.0.0.2:22: is running Ubuntu 20.10
INFO ==> Running phase: Prepare hosts
INFO ==> Running phase: Gather host facts
INFO [ssh] 10.0.0.1:22: discovered 10.12.18.133 as private address
INFO ==> Running phase: Validate hosts
INFO ==> Running phase: Gather k0s facts
INFO ==> Running phase: Download k0s binaries on hosts
INFO ==> Running phase: Configure k0s
INFO ==> Running phase: Initialize the k0s cluster
INFO [ssh] 10.0.0.1:22: installing k0s controller
INFO ==> Running phase: Install workers
INFO [ssh] 10.0.0.1:22: generating token
INFO [ssh] 10.0.0.2:22: installing k0s worker
INFO [ssh] 10.0.0.2:22: waiting for node to become ready
INFO ==> Running phase: Disconnect from hosts
INFO ==> Finished in 2m2s
INFO k0s cluster version 1.22.3+k0s.0 is now installed
INFO Tip: To access the cluster you can now fetch the admin kubeconfig using:
INFO      k0sctl kubeconfig
```

You can find example Terraform and [bootloose](https://github.com/k0sproject/bootloose) configurations in the [examples/](examples/) directory.

## Installation

### Install from the released binaries

Download the desired version for your operating system and processor architecture from the [k0sctl releases page](https://github.com/k0sproject/k0sctl/releases). Make the file executable and place it in a directory available in your `$PATH`.

As the released binaries aren't signed yet, on macOS and Windows, you must first run the executable via "Open" in the context menu and allow running it.

### Install from the sources

If you have a working Go toolchain, you can use `go install` to install k0sctl to your `$GOPATH/bin`.

```sh
go install github.com/k0sproject/k0sctl@latest
```

### Package managers

#### [Homebrew](https://brew.sh/) (macOS, Linux)

```sh
brew install k0sproject/tap/k0sctl
```

#### [Chocolatey](https://chocolatey.org/) (Windows)

Note: The [chocolatey package](https://community.chocolatey.org/packages/k0sctl) is community maintained, any issues should be reported to the maintainer of the package.

```sh
choco install k0sctl
```

### Container usage

It is possible to use `k0sctl` as a docker/OCI container:

```sh
# pull the image
docker pull ghcr.io/k0sproject/k0sctl:latest

# create a backup
docker run -it --workdir /backup \
  -v ./backup:/backup \
  -v ./k0sctl.yaml:/etc/k0s/k0sctl.yaml \
  ghcr.io/k0sproject/k0sctl:latest k0sctl backup --config /etc/k0s/k0sctl.yaml
```

#### Shell auto-completions

##### Bash

```sh
k0sctl completion > /etc/bash_completion.d/k0sctl
```

##### Zsh

```sh
k0sctl completion > /usr/local/share/zsh/site-functions/_k0sctl

# For oh my zsh
k0sctl completion > $ZSH_CACHE_DIR/completions/_k0sctl
```

##### Fish

```sh
k0sctl completion > ~/.config/fish/completions/k0sctl.fish
```

## Development status

K0sctl is ready for use and in continuous development.

### Contributing & Agent Guidelines

For repository layout, development, and testing guidelines (including notes for AI assistants), see [AGENTS.md](AGENTS.md).

## Usage

### `k0sctl apply`

The main function of k0sctl is the `k0sctl apply` subcommand. Provided a configuration file describing the desired cluster state, k0sctl will connect to the listed hosts, determines the current state of the hosts and configures them as needed to form a k0s cluster.

The default location for the configuration file is `k0sctl.yaml` in the current working directory. To load a configuration from a different location, use:

```sh
k0sctl apply --config path/to/k0sctl.yaml
```

If the configuration cluster version `spec.k0s.version` is greater than the version detected on the cluster, a cluster upgrade will be performed. If the configuration lists hosts that are not part of the cluster, they will be configured to run k0s and will be joined to the cluster.

### `k0sctl init`

Generate a configuration template. Use `--k0s` to include an example `spec.k0s.config` k0s configuration block. You can also supply a list of host addresses via arguments or stdin.

Output a minimal configuration template:

```sh
k0sctl init > k0sctl.yaml
```

Output an example configuration with a default k0s config:

```sh
k0sctl init --k0s > k0sctl.yaml
```

Create a configuration from a list of host addresses and pipe it to k0sctl apply:

```sh
k0sctl init 10.0.0.1 10.0.0.2 ubuntu@10.0.0.3:8022 | k0sctl apply --config -
```

### `k0sctl backup & restore`

Takes a [backup](https://docs.k0sproject.io/stable/backup/) of the cluster control plane state into the current working directory.

The files are currently named with a running (unix epoch) timestamp, e.g. `k0s_backup_1623220591.tar.gz`.

Restoring a backup can be done as part of the [k0sctl apply](#k0sctl-apply) command using `--restore-from k0s_backup_1623220591.tar.gz` flag.

Restoring the cluster state is a full restoration of the cluster control plane state, including:
- Etcd datastore content
- Certificates
- Keys

In general restore is intended to be used as a disaster recovery mechanism and thus it expects that no k0s components actually exist on the controllers.

Known limitations in the current restore process:
- The control plane address (`externalAddress`) needs to remain the same between backup and restore. This is caused by the fact that all worker node components connect to this address and cannot currently be re-configured.

### `k0sctl reset`

Uninstall k0s from the hosts listed in the configuration.

### `k0sctl kubeconfig`

Connects to the cluster and outputs a kubeconfig file that can be used with `kubectl` or `kubeadm` to manage the kubernetes cluster.

Example:

```sh
$ k0sctl kubeconfig --config path/to/k0sctl.yaml > k0s.config
$ kubectl get node --kubeconfig k0s.config
NAME      STATUS     ROLES    AGE   VERSION
worker0   NotReady   <none>   10s   v1.20.2-k0s1
```

## Configuration file

The configuration file is in YAML format and loosely resembles the syntax used in Kubernetes. YAML anchors and aliases can be used.

To generate a simple skeleton configuration file, you can use the `k0sctl init` subcommand.

For the complete configuration reference, see **[docs/configuration.md](docs/configuration.md)**.

A [JSON Schema](docs/k0sctl-schema.json) is also available for editor validation and autocompletion (e.g. VS Code YAML extension).

Configuration example:

```yaml
apiVersion: k0sctl.k0sproject.io/v1beta1
kind: Cluster
metadata:
  name: my-k0s-cluster
spec:
  hosts:
  - role: controller
    ssh:
      address: 10.0.0.1
      user: root
      keyPath: ~/.ssh/id_rsa
  - role: worker
    ssh:
      address: 10.0.0.2
  k0s:
    version: 1.32.2+k0s.0
```

### Environment variable substitution

Simple bash-like expressions are supported in the configuration for environment variable substitution.

- `$VAR` or `${VAR}` value of `VAR` environment variable
- `${var:-DEFAULT_VALUE}` will use `VAR` if non-empty, otherwise `DEFAULT_VALUE`
- `$$var` - escape, result will be `$var`.
- And [several other expressions](https://github.com/a8m/envsubst#docs)

## License
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fk0sproject%2Fk0sctl.svg?type=large)](https://app.fossa.com/projects/git%2Bgithub.com%2Fk0sproject%2Fk0sctl?ref=badge_large)
