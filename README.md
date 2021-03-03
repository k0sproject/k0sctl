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
$ GO111MODULE=on go get github.com/k0sproject/k0sctl
```

### Package managers

Scripts for installation via popular package managers such as Homebrew, Scoop or SnapCraft will be added later.

## Development status

K0sctl is still in an early stage of development. Missing major features include at least:

* Windows targets are not yet supported
* The released binaries have not been signed
* Cluster backup and restore are not available yet
* Nodes can't be removed

## Usage

### `k0sctl apply`

The main function of k0sctl is the `k0sctl apply` subcommand. Provided a configuration file describing the desired cluster state, k0sctl will connect to the listed hosts, determines the current state of the hosts and configures them as needed to form a k0s cluster.

The default location for the configuration file is `k0sctl.yaml` in the current working directory. To load a configuration from a different location, use:

```sh
$ k0sctl apply --config path/to/k0sctl.yaml
```

If the configuration cluster version `spec.k0s.version` is greater than the version detected on the cluster, a cluster upgrade will be performed. If the configuration lists hosts that are not part of the cluster, they will be configured to run k0s and will be joined to the cluster.

### `k0sctl init`

Generate a configuration template. Use `--k0s` to include an example `spec.k0s.config` k0s configuration block.

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

## Configuration file syntax

To generate a simple skeleton configuration file, you can use the `k0sctl init` subcommand:

```sh
$ k0sctl init > k0sctl.yaml
```

### Example configuration

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
      port: 22
      keyPath: ~/.ssh/id_rsa
  - role: worker
    ssh:
      address: 10.0.0.2
  k0s:
    version: 0.10.0
    instalFlags:
    - --debug
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

### Configuration file

The configuration file is in YAML format and loosely resembles the syntax used in Kubernetes. YAML anchors and aliases can be used.

#### `apiVersion` &lt;string&gt; (required)

The configuration file syntax version. Currently the only supported version is `k0sctl.k0sproject.io/v1beta1`.

#### `kind` &lt;string&gt; (required)

In the future, some of the configuration APIs can support multiple types of objects. For now, the only supported kind is `Cluster`.

### `metadata` &lt;mapping&gt; (optional)

Information that can be used to uniquely identify the object.

#### `metadata.name` &lt;string&gt; (optional) (default: `k0s-cluster`)

### `spec` &lt;mapping&gt; (required)

The object description.

### `spec.hosts` &lt;sequence&gt; (required)

A list of cluster hosts. Host requirements:

* Currently only linux targets are supported
* The user must either be root or have passwordless `sudo` access.
* The host must fulfill k0s system requirements

#### `spec.hosts[*].role` &lt;string&gt; (required)

One of `controller`, `worker` or to set up a controller that can also run workloads, use `controller+worker`.

#### `spec.hosts[*].uploadBinary` &lt;boolean&gt; (optional) (default: `false`)

When `true`, the k0s binaries for target host will be downloaded and cached on the local host and uploaded to the target.
When `false`, the k0s binary downloading is performed on the target host itself

#### `spec.hosts[*].k0sBinaryPath` &lt;string&gt; (optional)

A path to a file on the local host that contains a k0s binary to be uploaded to the host. Can be used to test drive a custom development build of k0s.

#### `spec.hosts[*].installFlags` &lt;sequence&gt; (optional)

Extra flags passed to the `k0s install` command on the target host. See `k0s install --help` for a list of options.

#### `spec.hosts[*].environment` &lt;mapping&gt; (optional)

List of key-value pairs to set to the target host's environment variables.

Example:

```yaml
environment:
  HTTP_PROXY: 10.0.0.1:443
```

#### `spec.hosts[*].ssh` &lt;mapping&gt; (optional)

SSH connection options.

##### `spec.hosts[*].ssh.address` &lt;string&gt; (required)

IP address of the host

##### `spec.hosts[*].ssh.user` &lt;string&gt; (optional) (default: `root`)

Username to log in as.

##### `spec.hosts[*].ssh.port` &lt;string&gt; (required)

TCP port of the SSH service on the host.

##### `spec.hosts[*].ssh.keyPath` &lt;string&gt; (optional) (default: `~/.ssh/id_rsa`)

Path to a SSH private key file.

#### `spec.hosts[*].localhost` &lt;mapping&gt; (optional)

Localhost connection options. Can be used to use the local host running k0sctl as a node in the cluster.

##### `spec.hosts[*].localhost.enabled` &lt;boolean&gt; (optional) (default: `false`)

This must be set `true` to enable the localhost connection.

### `spec.k0s` &lt;mapping&gt; (optional)

Settings related to the k0s cluster.

#### `spec.k0s.version` &lt;string&gt; (optional) (default: auto-discovery)

The version of k0s to deploy. When left out, k0sctl will default to using the latest released version of k0s or the version already running on the cluster.

#### `spec.k0s.config` &lt;mapping&gt; &lt;optional&gt; (default: auto-generated)

Embedded k0s cluster configuration. See [k0s configuration documentation](https://docs.k0sproject.io/main/configuration/) for details.

When left out, the output of `k0s default-config` will be used.
