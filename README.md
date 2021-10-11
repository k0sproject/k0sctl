# k0sctl

*A command-line bootstrapping and management tool for [k0s zero friction kubernetes](https://k0sproject.io/) clusters.*

Example output of k0sctl deploying a k0s cluster:

```sh
$ k0sctl apply

⠀⣿⣿⡇⠀⠀⢀⣴⣾⣿⠟⠁⢸⣿⣿⣿⣿⣿⣿⣿⡿⠛⠁⠀⢸⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⠀█████████ █████████ ███
⠀⣿⣿⡇⣠⣶⣿⡿⠋⠀⠀⠀⢸⣿⡇⠀⠀⠀⣠⠀⠀⢀⣠⡆⢸⣿⣿⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀███          ███    ███
⠀⣿⣿⣿⣿⣟⠋⠀⠀⠀⠀⠀⢸⣿⡇⠀⢰⣾⣿⠀⠀⣿⣿⡇⢸⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⠀███          ███    ███
⠀⣿⣿⡏⠻⣿⣷⣤⡀⠀⠀⠀⠸⠛⠁⠀⠸⠋⠁⠀⠀⣿⣿⡇⠈⠉⠉⠉⠉⠉⠉⠉⠉⢹⣿⣿⠀███          ███    ███
⠀⣿⣿⡇⠀⠀⠙⢿⣿⣦⣀⠀⠀⠀⣠⣶⣶⣶⣶⣶⣶⣿⣿⡇⢰⣶⣶⣶⣶⣶⣶⣶⣶⣾⣿⣿⠀█████████    ███    ██████████

INFO By continuing to use k0sctl you agree to these terms:
INFO https://k0sproject.io/licenses/eula
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
INFO [ssh] 10.0.0.1:22: validating configuration
INFO ==> Running phase: Initialize the k0s cluster
INFO [ssh] 10.0.0.1:22: installing k0s controller
INFO ==> Running phase: Install workers
INFO [ssh] 10.0.0.1:22: generating token
INFO [ssh] 10.0.0.2:22: installing k0s worker
INFO [ssh] 10.0.0.2:22: waiting for node to become ready
INFO ==> Running phase: Disconnect from hosts
INFO ==> Finished in 2m2s
INFO k0s cluster version 0.11.0 is now installed
INFO Tip: To access the cluster you can now fetch the admin kubeconfig using:
INFO      k0sctl kubeconfig
```

## Installation

### Install from the released binaries

Download the desired version for your operating system and processor architecture from the [k0sctl releases page](https://github.com/k0sproject/k0sctl/releases). Make the file executable and place it in a directory available in your `$PATH`.

As the released binaries aren't signed yet, on macOS and Windows, you must first run the executable via "Open" in the context menu and allow running it.

### Install from the sources

If you have a working Go toolchain, you can use `go get` to install k0sctl to your `$GOPATH/bin`.

```sh
GO111MODULE=on go get github.com/k0sproject/k0sctl
```

### Package managers

Scripts for installation via popular package managers such as Homebrew, Scoop or SnapCraft will be added later.

## Development status

K0sctl is still in an early stage of development. Missing major features include at least:

* Windows targets are not yet supported
* The released binaries have not been signed
* Nodes can't be removed

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

Takes a [backup](https://docs.k0sproject.io/main/backup/) of the cluster control plane state into the current working directory.

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
      kind: Cluster
      metadata:
        name: my-k0s-cluster
      images:
        calico:
          cni:
            image: calico/cni
            version: v3.16.2
```

### Configuration Header Fields

###### `apiVersion` &lt;string&gt; (required)

The configuration file syntax version. Currently the only supported version is `k0sctl.k0sproject.io/v1beta1`.

###### `kind` &lt;string&gt; (required)

In the future, some of the configuration APIs can support multiple types of objects. For now, the only supported kind is `Cluster`.

###### `spec` &lt;mapping&gt; (required)

The main object definition, see [below](#configuration-spec)

###### `metadata` &lt;mapping&gt; (optional)

Information that can be used to uniquely identify the object.

Example:

```yaml
metadata:
  name: k0s-cluster-name
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

See [k0s object documentation](#spec-fields) below.

### Host Fields

###### `spec.hosts[*].role` &lt;string&gt; (required)

One of `controller`, `worker` or to set up a controller that can also run workloads, use `controller+worker`.

###### `spec.hosts[*].uploadBinary` &lt;boolean&gt; (optional) (default: `false`)

When `true`, the k0s binaries for target host will be downloaded and cached on the local host and uploaded to the target.
When `false`, the k0s binary downloading is performed on the target host itself

###### `spec.hosts[*].k0sBinaryPath` &lt;string&gt; (optional)

A path to a file on the local host that contains a k0s binary to be uploaded to the host. Can be used to test drive a custom development build of k0s.

###### `spec.hosts[*].hostname` &lt;string&gt; (optional)

Override host's hostname. When not set, the hostname reported by the operating system is used.

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
  perm: 0700
```

* `name`: name of the file "bundle", used only for logging purposes (optional)
* `src`: [Glob pattern](https://golang.org/pkg/path/filepath/#Match) to match files to be uploaded
* `dstDir`: Destination directory for the file(s). `k0sctl` will create full directory structure if it does not already exist on the host.
* `perm`: File permission mode for uploaded file(s) and created directories

###### `spec.hosts[*].hooks` &lt;mapping&gt; (optional)

Run a set of commands on the remote host during k0sctl operations.

Example:

```yaml
hooks:
  apply:
    before:
      - date > k0sctl-apply.log
    after:
      - echo "apply success" > k0sctl-apply.log
```

The currently available "hook points" are:

* `apply`: Runs during `k0sctl apply`
    - `before`: Runs after configuration and host validation, right before configuring k0s on the host
    - `after`: Runs before disconnecting from the hosts after a successful apply operation
* `backup`: Runs during `k0s backup`
    - `before`: Runs before k0sctl runs the `k0s backup` command
    - `after`: Runs before disconnecting from the hosts after successfully taking a backup
* `reset`: Runs during `k0sctl reset`
    - `before`: Runs after gathering information about the cluster, right before starting to remove the k0s installation.
    - `after`: Runs before disconnecting from the hosts after a successful reset operation

##### `spec.hosts[*].os` &lt;string&gt; (optional) (default: ``)

Override OS distribution auto-detection. By default `k0sctl` detects the OS by reading `/etc/os-release` or `/usr/lib/os-release` files. In case your system is based on e.g. Debian but the OS release info has something else configured you can override `k0sctl` to use Debian based functionality for the node with:
```yaml
  - role: worker
    os: debian
    ssh:
      address: 10.0.0.2
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

```
$ ssh-add ~/.ssh/aws.pem
$ ssh -A user@jumphost
user@jumphost ~ $ k0sctl apply
```

###### `spec.hosts[*].ssh.address` &lt;string&gt; (required)

IP address of the host

###### `spec.hosts[*].ssh.user` &lt;string&gt; (optional) (default: `root`)

Username to log in as.

###### `spec.hosts[*].ssh.port` &lt;string&gt; (required)

TCP port of the SSH service on the host.

###### `spec.hosts[*].ssh.keyPath` &lt;string&gt; (optional) (default: `~/.ssh/id_rsa`)

Path to a SSH private key file.

##### `spec.hosts[*].localhost` &lt;mapping&gt; (optional)

Localhost connection options. Can be used to use the local host running k0sctl as a node in the cluster.

###### `spec.hosts[*].localhost.enabled` &lt;boolean&gt; (optional) (default: `false`)

This must be set `true` to enable the localhost connection.

### K0s Fields

##### `spec.k0s.version` &lt;string&gt; (optional) (default: auto-discovery)

The version of k0s to deploy. When left out, k0sctl will default to using the latest released version of k0s or the version already running on the cluster.

##### `spec.k0s.config` &lt;mapping&gt; (optional) (default: auto-generated)

Embedded k0s cluster configuration. See [k0s configuration documentation](https://docs.k0sproject.io/main/configuration/) for details.

When left out, the output of `k0s default-config` will be used.
