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
docker pull ghcr.io/k0sprojects/k0sctl:latest

# create a backup
docker run -it --workdir /backup \
  -v ./backup:/backup \
  -v ./k0sctl.yaml:/etc/k0s/k0sctl.yaml \
  ghcr.io/k0sprojects/k0sctl:latest k0sctl backup --config /etc/k0s/k0sctl.yaml
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

Configuration example:

```yaml
apiVersion: k0sctl.k0sproject.io/v1beta1
kind: Cluster
metadata:
  name: my-k0s-cluster
  user: admin
spec:
  hosts:
  - role: controller
    installFlags:
    - --debug
    ssh:
      address: 10.0.0.1
      user: root
      port: 22
      keyPath: ~/.ssh/id_rsa
  - role: worker
    installFlags:
    - --debug
    ssh:
      address: 10.0.0.2
  k0s:
    version: 0.10.0
    config:
      apiVersion: k0s.k0sproject.io/v1beta1
      kind: ClusterConfig
      metadata:
        name: my-k0s-cluster
      spec:
        images:
          calico:
            cni:
              image: calico/cni
              version: v3.16.2
  options:
    wait:
      enabled: true
    drain:
      enabled: true
    evictTaint:
      enabled: false
      taint: k0sctl.k0sproject.io/evict=true
      effect: NoExecute
    concurrency:
      limit: 30
      uploads: 5
```

### Environment variable substitution

Simple bash-like expressions are supported in the configuration for environment variable substition.

- `$VAR` or `${VAR}` value of `VAR` environment variable
- `${var:-DEFAULT_VALUE}` will use `VAR` if non-empty, otherwise `DEFAULT_VALUE`
- `$$var` - escape, result will be `$var`.
- And [several other expressions](https://github.com/a8m/envsubst#docs)

### Configuration Header Fields

###### `apiVersion` &lt;string&gt; (required)

The configuration file syntax version. Currently the only supported version is `k0sctl.k0sproject.io/v1beta1`.

###### `kind` &lt;string&gt; (required)

In the future, some of the configuration APIs can support multiple types of objects. For now, the only supported kind is `Cluster`.

###### `spec` &lt;mapping&gt; (required)

The main object definition, see [below](#spec-fields)

###### `metadata` &lt;mapping&gt; (optional)

Information that can be used to uniquely identify the object.

Example:

```yaml
metadata:
  name: k0s-cluster-name
  user: kubernetes-admin
```

### Spec Fields

##### `spec.hosts` &lt;sequence&gt; (required)

A list of cluster hosts. Host requirements:

* Currently only linux targets are supported
* The user must either be root or have passwordless `sudo` access.
* The host must fulfill the k0s system requirements

See [host object documentation](#host-fields) below.

##### `spec.k0s` &lt;mapping&gt; (optional)

Settings related to the k0s cluster.

See [k0s object documentation](#k0s-fields) below.

### Host Fields

###### `spec.hosts[*].role` &lt;string&gt; (required)

One of:
- `controller` - a controller host
- `controller+worker` - a controller host that will also run workloads
- `single` - a [single-node cluster](https://docs.k0sproject.io/stable/k0s-single-node/) host, the configuration can only contain one host
- `worker` - a worker host

###### `spec.hosts[*].noTaints` &lt;boolean&gt; (optional) (default: `false`)

When `true` and used in conjuction with the `controller+worker` role, the default taints are disabled making regular workloads schedulable on the node. By default, k0s sets a node-role.kubernetes.io/master:NoSchedule taint on controller+worker nodes and only workloads with toleration for it will be scheduled.

###### `spec.hosts[*].uploadBinary` &lt;boolean&gt; (optional) (default: `false`)

When `true`, the k0s binaries for target host will be downloaded and cached on the local host and uploaded to the target.
When `false`, the k0s binary downloading is performed on the target host itself

###### `spec.hosts[*].k0sBinaryPath` &lt;string&gt; (optional)

A path to a file on the local host that contains a k0s binary to be uploaded to the host. Can be used to test drive a custom development build of k0s.

###### `spec.hosts[*].k0sInstallPath` &lt;string&gt; (optional) (default: depends on OS)

A path on the node where to install the k0s binary.

###### `spec.hosts[*].k0sDownloadURL` &lt;string&gt; (optional)

A URL to download the k0s binary from. The default is to download from the [k0s repository](https://github.com/k0sproject/k0s). The URL can contain '%'-prefixed tokens that will be replaced with the host's information, see [tokens](#tokens).

###### `spec.hosts[*].hostname` &lt;string&gt; (optional)

Override host's hostname. When not set, the hostname reported by the operating system is used.

###### `spec.hosts[*].dataDir` &lt;string&gt; (optional) (default: `/var/lib/k0s`)

Set host's k0s data-dir.

###### `spec.hosts[*].kubeletRootDir` &lt;string&gt; (optional) (default: `""`)

Set host's k0s kubelet-root-dir.

###### `spec.hosts[*].installFlags` &lt;sequence&gt; (optional)

Extra flags passed to the `k0s install` command on the target host. See `k0s install --help` for a list of options.

###### `spec.hosts[*].environment` &lt;mapping&gt; (optional)

List of key-value pairs to set to the target host's environment variables.

Example:

```yaml
environment:
  HTTP_PROXY: 10.0.0.1:443
```

###### `spec.hosts[*].files` &lt;sequence&gt; (optional)

List of files to be uploaded to the host.

Example:

```yaml
- name: image-bundle
  src: airgap-images.tgz
  dstDir: /var/lib/k0s/images/
  perm: 0600
```

Inline data example:

```yaml
- name: motd
  data: |
    Powered by k0s
  dst: /etc/motd
  perm: 0644
```

* `name`: name of the file "bundle", used only for logging purposes (optional)
* `src`: File path, an URL or [Glob pattern](https://golang.org/pkg/path/filepath/#Match) to match files to be uploaded. URL sources will be directly downloaded using the target host. If the value is a URL, '%'-prefixed tokens can be used, see [tokens](#tokens). (required when `data` is not set)
* `data`: Inline file data to write to the destination. Use together with `dst` or `dst` + `dstDir`. (required when `src` is not set)
* `dstDir`: Destination directory for the file(s). `k0sctl` will create full directory structure if it does not already exist on the host (default: user home)
* `dst`: Destination filename for the file. Only usable for single file uploads (default: basename of file)
* `perm`: File permission mode for uploaded file(s) (default: same as local)
* `dirPerm`: Directory permission mode for created directories (default: 0755)
* `user`: User name of file/directory owner, must exist on the host (optional)
* `group`: Group name of file/directory owner, must exist on the host (optional)

###### `spec.hosts[*].hooks` &lt;mapping&gt; (optional)

Run a set of commands on the remote host during k0sctl operations.

Example:

```yaml
hooks:
  connect:
    after:
      - echo "connected and detected" >> k0sctl-connect.log
  upgrade:
    before:
      - echo "about to upgrade" >> k0sctl-upgrade.log
    after:
      - echo "upgraded" >> k0sctl-upgrade.log
  apply:
    before:
      - date >> k0sctl-apply.log
    after:
      - echo "apply success" >> k0sctl-apply.log
```

The currently available "hook points" are:

* `connect`: 
    - `after`: Runs immediately after OS detection completes
* `apply`: Runs during `k0sctl apply`
    - `before`: Runs after configuration and host validation, right before configuring k0s on the host
    - `after`: Runs before disconnecting from the host after a successful apply operation
* `upgrade`: Runs during `k0sctl apply`
    - `before`: Runs for each host that is going to be upgraded, before the upgrade begins
    - `after`: Runs for each host that was upgraded, after the upgrade completes
* `install`: Runs during `k0sctl apply`
    - `before`: Runs on each host just before installing its k0s components. This includes the first controller (Initialize the k0s cluster), additional controllers, and workers.
    - `after`: Runs on each host immediately after installing its k0s components (service started and ready checks done).
* `backup`: Runs during `k0s backup`
    - `before`: Runs before k0sctl runs the `k0s backup` command
    - `after`: Runs before disconnecting from the host after successfully taking a backup
* `reset`: Runs during `k0sctl reset` or when `k0sctl apply` resets a host.
    - `before`: Runs after gathering information about the cluster, right before starting to remove the k0s installation.
    - `after`: Runs before disconnecting from the host after a successful reset operation

Notes:

- Hooks run on each host that defines them, using the same remote user as the connection. If elevated privileges are required, prefix commands with `sudo`.
- In dry-run mode, hooks are not executed; k0sctl prints what would run on each host.
- Hooks execute only on hosts targeted by the related phase. For example, `upgrade` hooks run only for hosts that need upgrade.

##### `spec.hosts[*].os` &lt;string&gt; (optional) (default: ``)

Override OS distribution auto-detection. By default `k0sctl` detects the OS by reading `/etc/os-release` or `/usr/lib/os-release` files. In case your system is based on e.g. Debian but the OS release info has something else configured you can override `k0sctl` to use Debian based functionality for the node with:

```yaml
  - role: worker
    os: debian
    ssh:
      address: 10.0.0.2
```

##### `spec.hosts[*].privateInterface` &lt;string&gt; (optional) (default: ``)

Override private network interface selected by host fact gathering.
Useful in case fact gathering picks the wrong private network interface.

```yaml
  - role: worker
    os: debian
    privateInterface: eth1
```

##### `spec.hosts[*].privateAddress` &lt;string&gt; (optional) (default: ``)

Override private IP address selected by host fact gathering.
Useful in case fact gathering picks the wrong IPAddress.

```yaml
  - role: worker
    os: debian
    privateAddress: 10.0.0.2
```

##### `spec.hosts[*].ssh` &lt;mapping&gt; (optional)

SSH connection options.

Example:

```yaml
spec:
  hosts:
    - role: controller
      ssh:
        address: 10.0.0.2
        user: ubuntu
        keyPath: ~/.ssh/id_rsa
```

It's also possible to tunnel connections through a bastion host. The bastion configuration has all the same fields as any SSH connection:

```yaml
spec:
  hosts:
    - role: controller
      ssh:
        address: 10.0.0.2
        user: ubuntu
        keyPath: ~/.ssh/id_rsa
        bastion:
          address: 10.0.0.1
          user: root
          keyPath: ~/.ssh/id_rsa2
```

SSH agent and auth forwarding are also supported, a host without a keyfile:

```yaml
spec:
  hosts:
    - role: controller
      ssh:
        address: 10.0.0.2
        user: ubuntu
```

```shell
$ ssh-add ~/.ssh/aws.pem
$ ssh -A user@jumphost
user@jumphost ~ $ k0sctl apply
```

Pageant or openssh-agent can be used on Windows.

###### `spec.hosts[*].ssh.address` &lt;string&gt; (required)

IP address of the host

###### `spec.hosts[*].ssh.user` &lt;string&gt; (optional) (default: `root`)

Username to log in as.

###### `spec.hosts[*].ssh.port` &lt;number&gt; (required)

TCP port of the SSH service on the host.

###### `spec.hosts[*].ssh.keyPath` &lt;string&gt; (optional) (default: `~/.ssh/identity ~/.ssh/id_rsa ~/.ssh/id_dsa`)

Path to an SSH key file. If a public key is used, ssh-agent is required. When left empty, the default value will first be looked for from the ssh configuration (default `~/.ssh/config`) `IdentityFile` parameter.

##### `spec.hosts[*].localhost` &lt;mapping&gt; (optional)

Localhost connection options. Can be used to use the local host running k0sctl as a node in the cluster.

###### `spec.hosts[*].localhost.enabled` &lt;boolean&gt; (optional) (default: `false`)

This must be set `true` to enable the localhost connection.

##### `spec.hosts[*].openSSH` &lt;mapping&gt; (optional)

An alternative SSH client protocol that uses the system's openssh client for connections.

Example:

```yaml
spec:
  hosts:
    - role: controller
      openSSH:
        address: 10.0.0.2
```

The only required field is the `address` and it can also be a hostname that is found in the ssh config. All other options such as user, port and keypath will use the same defaults as if running `ssh` from the command-line or will use values found from the ssh config.

An example SSH config:

```
Host controller1
  Hostname 10.0.0.1
  Port 2222
  IdentityFile ~/.ssh/id_cluster_esa
```

If this is in your `~/.ssh/config`, you can simply use the host alias as the address in your k0sctl config:

```yaml
spec:
  hosts:
    - role: controller
      openSSH:
        address: controller1
        # if the ssh configuration is in a different file, you can use:
        # configPath: /path/to/config
```

###### `spec.hosts[*].openSSH.address` &lt;string&gt; (required)

IP address, hostname or ssh config host alias of the host

###### `spec.hosts[*].openSSH.user` &lt;string&gt; (optional)

Username to connect as.

###### `spec.hosts[*].openSSH.port` &lt;number&gt; (optional)

Remote port.

###### `spec.hosts[*].openSSH.keyPath` &lt;string&gt; (optional)

Path to private key.

###### `spec.hosts[*].openSSH.configPath` &lt;string&gt; (optional)

Path to ssh config, defaults to ~/.ssh/config with fallback to /etc/ssh/ssh_config.

###### `spec.hosts[*].openSSH.disableMultiplexing` &lt;boolean&gt; (optional)

The default mode of operation is to use connection multiplexing where a ControlMaster connection is opened and the subsequent connections to the same host use the master connection over a socket to communicate to the host. 

If this is disabled by setting `disableMultiplexing: true`, running every remote command will require reconnecting and reauthenticating to the host.

###### `spec.hosts[*].openSSH.options` &lt;mapping&gt; (optional)

Additional options as key/value pairs to use when running the ssh client.

Example:

```yaml
openSSH:
  address: host
  options:
      ForwardAgent: true  # -o ForwardAgent=yes
      StrictHostkeyChecking: false # -o StrictHostkeyChecking: no
```

###### `spec.hosts[*].reset` &lt;boolean&gt; (optional) (default: `false`)

If set to `true` k0sctl will remove the node from kubernetes and reset k0s on the host.

### K0s Fields

##### `spec.k0s.version` &lt;string&gt; (optional) (default: auto-discovery)

The version of k0s to deploy. When left out, k0sctl will default to using the latest released version of k0s or the version already running on the cluster.

##### `spec.k0s.versionChannel` &lt;string&gt; (optional) (default: `stable`)

Possible values are `stable` and `latest`.

When `spec.k0s.version` is left undefined, this setting can be set to `latest` to allow k0sctl to include k0s pre-releases when looking for the latest version. The default is to only look for stable releases.

##### `spec.k0s.dynamicConfig` &lt;boolean&gt; (optional) (default: false)

Enable k0s dynamic config. The setting will be automatically set to true if:

* Any controller node has `--enable-dynamic-config` in `installFlags`
* Any existing controller node has `--enable-dynamic-config` in run arguments (`k0s status -o json`)

**Note:** When running k0s in dynamic config mode, k0sctl will ONLY configure the cluster-wide configuration during the first time initialization, after that the configuration has to be managed via `k0s config edit` or `k0sctl config edit`. The node specific configuration will be updated on each apply.

See also:

* [k0s Dynamic Configuration](https://docs.k0sproject.io/stable/dynamic-configuration/)

##### `spec.k0s.config` &lt;mapping&gt; (optional) (default: auto-generated)

Embedded k0s cluster configuration. See [k0s configuration documentation](https://docs.k0sproject.io/stable/configuration/) for details.

When left out, the output of `k0s config create` will be used.

You can also host the configuration in a separate file or as a separate YAML document in the same file in the standard k0s configuration format.

```yaml
apiVersion: k0sctl.k0sproject.io/v1beta1
kind: Cluster
spec:
  hosts:
    - role: single
      ssh:
        address: 10.0.0.1
---
apiVersion: k0s.k0sproject.io/v1beta1
kind: ClusterConfig
metadata:
  name: my-k0s-cluster
spec:
  api:
    externalAddress: 10.0.0.2
```

### Options Fields

The `spec.options` field contains options that can be used to modify the behavior of k0sctl.

Example:

```yaml
spec:
  options:
    wait:
      enabled: true
    drain:
      enabled: true
    evictTaint:
      enabled: false
      taint: k0sctl.k0sproject.io/evict=true
      effect: NoExecute
    concurrency:
      limit: 30
      workerDisruptionPercent: 10
      uploads: 5
```

##### `spec.options.wait.enabled` &lt;boolean&gt; (optional) (default: true)

If set to `false`, k0sctl will not wait for k0s to become ready after restarting the service. By default, k0sctl waits for nodes to become ready before continuing to the next operation. This is functionally the same as using `--no-wait` on the command line.

##### `spec.options.drain.enabled` &lt;boolean&gt; (optional) (default: true)

If set to `false`, k0sctl will skip draining nodes before performing disruptive operations like upgrade or reset. By default, nodes are drained to allow for graceful pod eviction. This is functionally the same as using `--no-drain` on the command line.

##### `spec.options.drain.gracePeriod` &lt;duration&gt; (optional) (default: 2m)

The duration to wait for pods to be evicted from the node before proceeding with the operation. 

##### `spec.options.drain.timeout` &lt;duration&gt; (optional) (default: 5m)

The duration to wait for the drain operation to complete before timing out. 

##### `spec.options.drain.force` &lt;boolean&gt; (optional) (default: true)

Use `--force` in kubectl when draining the node.

##### `spec.options.drain.ignoreDaemonSets` &lt;boolean&gt; (optional) (default: true)

Ignore DaemonSets when draining the node.

##### `spec.options.drain.deleteEmptyDirData` &lt;boolean&gt; (optional) (default: true)

Continue even if there are pods using emptyDir (local data that will be deleted when the node is drained).

##### `spec.options.drain.skipWaitForDeleteTimeout` &lt;duration&gt; (optional) (default: 0s)

If pod DeletionTimestamp older than N seconds, skip waiting for the pod. Seconds must be greater than 0 to skip.

##### `spec.options.drain.podSelector` &lt;string&gt; (optional) (default: ``)

Label selector to filter pods on the node

##### `spec.options.evictTaint.enabled` &lt;boolean&gt; (optional) (default: false)

When enabled, k0sctl will apply a taint to nodes before service-affecting operations such as upgrade or reset. This is used to signal workloads to be evicted in advance of node disruption. You can also use the `--evict-taint=k0sctl.k0sproject.io/evic=true:NoExecute` command line option to enable this feature.

##### `spec.options.evictTaint.taint` &lt;string&gt; (optional) (default: `k0sctl.k0sproject.io/evict=true`)

The taint to apply when `evictTaint.enabled` is `true`. Must be in the format `key=value`.

##### `spec.options.evictTaint.effect` &lt;string&gt; (optional) (default: `NoExecute`)

The taint effect to apply. Must be one of:

* `NoExecute`
* `NoSchedule`
* `PreferNoSchedule`

##### `spec.options.evictTaint.controllerWorkers` &lt;boolean&gt; (optional) (default: false)

Whether to also apply the taint to nodes with the controller+worker dual role. By default, taints are only applied to worker-only nodes.

##### `spec.options.concurrency.limit` &lt;integer&gt; (optional) (default: 30)

The maximum number of hosts to operate on concurrently during cluster operations. Same as the `--concurrency` command line option.

##### `spec.options.concurrency.workerDisruptionPercent` &lt;integer&gt; (optional) (default: 10)

The maximum percentage of worker nodes that can be disrupted at the same time during operations such as upgrade. This is used to ensure that a minimum number of worker nodes remain available during the operation. The value must be between 0 and 100.

##### `spec.options.concurrency.uploads` &lt;integer&gt; (optional) (default: 5)

The maximum number of concurrent file uploads to perform. Same as the `--concurrent-uploads` command line option.

### Tokens

The following tokens can be used in the `k0sDownloadURL` and `files.[*].src` fields:

- `%%` - literal `%`
- `%p` - host architecture (arm, arm64, amd64)
- `%v` - k0s version (v1.21.0+k0s.0)
- `%x` - k0s binary extension (.exe on Windows, empty elsewhere)

Any other tokens will be output as-is including the `%` character.

Example:

```yaml
  - role: controller
    k0sDownloadURL: https://files.example.com/k0s%20files/k0s-%v-%p%x
    # Expands to https://files.example.com/k0s%20files/k0s-v1.21.0+k0s.0-amd64
```
## License
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fk0sproject%2Fk0sctl.svg?type=large)](https://app.fossa.com/projects/git%2Bgithub.com%2Fk0sproject%2Fk0sctl?ref=badge_large)
