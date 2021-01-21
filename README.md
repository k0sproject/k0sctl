
- [k0sctl - k0s tool](#k0sctl---k0s-tool)
- [Installation](#installation)
    - [Install binary from Source](#install-binary-from-source)
    - [Install binary from Release Download](#install-binary-from-release-download)
    - [Configuring your Path](#configuring-your-path)
  - [Linux and MacOs](#linux-and-macos)
  - [Windows](#windows)
  - [Running k0sctl](#running-k0sctl)


# k0sctl - k0s tool

k0sctl is k0s tool that allows users to easily deploy and manage k0s cluster.

# Installation

### Install binary from Source

Download the appropriate package, build and install them. Type the following in your terminal:

```
GO111MODULE=on go get github.com/k0sproject/k0sctl
```

You can find the installed executable/binary in `$GOPATH/bin` directory.


### Install binary from Release Download

Download the desired binary for your platform to the desired location from [here](https://github.com/k0sproject/k0sctl/tags) 

### Configuring your Path

If the directory where your binaries were installed is not already in your `PATH` environment variable, then it will need to be added.
Choose the steps to follow for your platform to add directory to `PATH`.


## Linux and MacOs

If you want to run k0sctl in a shell on Linux and placed the binary in `/home/YOUR-USER-NAME/k0sctl`, then type the following into your terminal:

```
export PATH=$PATH:/home/$USER/k0sctl
```

## Windows

If you want to run k0sctlPowerShell on Windows and placed the binary in `c:\k0sctl`, then type the following into PowerShell:

```
$env:Path += ";c:\k0sctl"
```

## Running k0sctl 

k0sctl allows users to bootstrap k0s cluster based on provided configuration. 
Example:

```
apiVersion: k0sctl.k0sproject.io/v1beta1
kind: cluster
spec:
  hosts:
    - role: server
      ssh:
        address: 127.0.0.1
        port: 9022
    - role: worker
      ssh:
        address: 127.0.0.1
        port: 9023
  k0s:
    version: 0.10.0
```

* `hosts` - information about remote hosts where k0s will be installed
  * `role` - (string) sets role of the k0s nodes possible roles are server or worker
  * `ssh` - parameters needed to establish secure shell connection for the given host
    * `address` - (string) IP address of the remote host
    * `port` - (integer) ssh port
    * `keyPath` - (string) path to the RSA key
    * `user` - (string) user name
  * `winRM` - parameters needed to establis winRM session
    * `address` - (string) IP address of the remote host
    * `port` - (integrer) winRM port
    * `keyPath` - (string) path to the RSA key
    * `user` - (string) user name
    * `useHTTPS` - (bool)
    * `insecure` - (bool)
    * `useNTLM` - (bool)
    * `caCertPath` - (string)
    * `certPath` - (string)
    * `tlsServerName` - (string)
  * `localhost` 
    * `enabled` - (bool)
* `k0s` - this section holds information about desired k0s setup
  * `version` -  (string) version of the k0s binary if not provided k0sctl will pull the latest version. Note: only supports versions =>0.10.0
  * `config` - k0s specific [configuration](https://github.com/k0sproject/k0s/blob/main/docs/configuration.md) if not provided k0s will run with default values.

