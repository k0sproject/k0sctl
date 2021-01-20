
- [k0sctl - k0s tool](#k0sctl---k0s-tool)
- [Installation](#installation)
    - [Install binary from Source](#install-binary-from-source)
    - [Install binary from Release Download](#install-binary-from-release-download)
    - [Configuring your Path](#configuring-your-path)
  - [Linux and MacOs](#linux-and-macos)
  - [Windows](#windows)


# k0sctl - k0s tool

k0sctl is k0s tool that allows users to easily deploy k0s cluster.


# Installation

### Install binary from Source

Download the appropriate  package, build and install them. Type the following in your terminal:

```
GO111MODULE=on go get github.com/k0sproject/k0sctl
```

You can find the installed executable/binary in `$GOPATH/bin` directory.


### Install binary from Release Download

Download the desired binary for your platform to the desired location from (here)[https://github.com/k0sproject/k0sctl/tags] 

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

