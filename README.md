# k0sctl

*A bootstrapping and management command-line tool for [k0s zero friction kubernetes](https://https://k0sproject.io/) clusters.*

## Installation

### Install from the released binaries

Download the desired version for your operating system and processor architecture from the [k0sctl releases page](https://github.com/k0sproject/k0sctl/releases). Make the file executable and place it in a directory available in your `$PATH`.

As the released binaries aren't signed yet, on macOS and Windows, you must first run the executable via "Open" in the context menu and allow running it.

### Install from the sources

If you have a working Go toolchain, you can use `go get` to install k0sctl to your `$GOPATH/bin`.

```
$ GO111MODULE=on go get github.com/k0sproject/k0sctl
```

### Package managers

Scripts for installation via popular package managers such as Homebrew, Scoop or SnapCraft will be added later.

## Development status

K0sctl is still in an early stage of development. Missing major features include at least:

* Cluster upgrades are not yet possible
* Windows targets are not yet supported
* The released binaries have not been signed
* Cluster uninstall and host clean up after failure is not there yet
* Cluster backup and restore are not available yet

## Usage

The main function of k0sctl is the `k0sctl apply` subcommand. Provided a configuration file, k0sctl will connect to the listed hosts and install k0s on them.

The default location for the configuration file is `k0sctl.yaml` in the current working directory. To load a configuration from a different location, use:

```
$ k0sctl apply --config path/to/k0sctl.yaml
```

## Configuration file syntax

To generate a simple skeleton configuration file, you can use the `k0sctl init` subcommand:

```
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
  - role: server
    ssh:
      address: 10.0.0.1
      user: root
      port: 22
      keyPath: ~/.ssh/id_rsa
  - role: worker
    ssh:
      address: 10.0.0.2
      user: root
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

### Configuration file `spec` fields

* `hosts` List of target hosts
  * `role` One of `server`, `worker` or if you want the server to run workloads, use `server+worker`
  * `uploadBinary` When set to `true`, instead of having the hosts download the k0s binaries from the internet, k0sctl will download them to the local storage and upload to the target hosts.
  * `k0sBinaryPath` Use a k0s binary from a local path on the host, useful for example to run a locally compiled development version.
  * `installFlags` a list of extra arguments passed to the `k0s install` command. See [k0s install command documentation](https://docs.k0sproject.io/main/cli/k0s_install/) for details.
  * `ssh` SSH connection parameters
    * `address` IP address or hostname of the remote host
    * `port` SSH port, default is 22.
    * `keyPath` Path to a SSH private key file, default is `~/.ssh/id_rsa`
    * `user` Username to connect as, the default is `root`
  * `localhost` You can use the local host that is running k0sctl as a node in the host
    * `enabled` Set this to true to enable the localhost connection. You can leave out the SSH configuration.
* `k0s` K0s options
  * `version` Target k0s version. Default is to use the latest released version.
  * `config` An embedded k0s cluster configuration. See [k0s configuration documentation](https://docs.k0sproject.io/main/configuration/) for details.
