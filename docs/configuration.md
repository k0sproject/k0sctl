# k0sctl Configuration Reference

> This document is auto-generated from the Go struct definitions. To regenerate, run `make docs`.

The configuration file is in YAML format and loosely resembles the syntax used in Kubernetes.
YAML anchors and aliases can be used.

Use `k0sctl init` to generate a skeleton configuration file.

## Example

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
      user: root
      keyPath: ~/.ssh/id_rsa
  k0s:
    version: 1.32.2+k0s.0
  options:
    wait:
      enabled: true
    drain:
      enabled: true
    evictTaint:
      enabled: false
    concurrency:
      limit: 30
      workerDisruptionPercent: 10
      uploads: 5
```

## Configuration Fields

**`apiVersion`** <string> (required) — Configuration file syntax version. Must be k0sctl.k0sproject.io/v1beta1.

**`kind`** <string> (required) — Object kind. Must be Cluster.

**`metadata`** <object> (optional) — Information that can be used to uniquely identify the object.

**`spec`** <object> (required) — Cluster specification.

## `metadata`

**`metadata.name`** <string> (optional) (default: `k0s-cluster`) — Name of the cluster.

**`metadata.user`** <string> (optional) (default: `admin`) — Kubernetes admin user name.

## `spec`

**`spec.hosts`** <object[]> (required) — A list of cluster hosts.

**`spec.k0s`** <object> (optional) — Settings related to the k0s cluster.

**`spec.options`** <object> (required) — Options for cluster operations.

### `spec.hosts[*]`

**`spec.hosts[*].winRM`** <object> (optional)

**`spec.hosts[*].ssh`** <object> (optional)

**`spec.hosts[*].localhost`** <object> (optional)

**`spec.hosts[*].openSSH`** <object> (optional)

**`spec.hosts[*].role`** <string> (required) — Role of the host in the cluster. One of:
  - controller — a controller-only node
  - controller+worker — a controller that also runs workloads
  - single — a single-node cluster; the configuration may only contain one host with this role
  - worker — a worker node

**`spec.hosts[*].reset`** <boolean> (optional) — When true, k0sctl will remove the node from Kubernetes and reset k0s on the host.

**`spec.hosts[*].privateInterface`** <string> (optional) — Override the private network interface selected by host fact gathering. Useful when
fact gathering picks the wrong interface for intra-cluster communication.

**`spec.hosts[*].privateAddress`** <string> (optional) — Override the private IP address selected by host fact gathering. Useful when
fact gathering picks the wrong address for intra-cluster communication.

**`spec.hosts[*].dataDir`** <string> (optional) (default: `/var/lib/k0s`) — Override the k0s data directory on the host.

**`spec.hosts[*].kubeletRootDir`** <string> (optional) — Override the kubelet root directory on the host.

**`spec.hosts[*].environment`** <object> (optional) — Environment variables to set in the k0s service environment on the host.

**`spec.hosts[*].uploadBinary`** <boolean> (optional) — When true, the k0s binary is downloaded on the local machine and uploaded to the
target host. When false (the default), the binary is downloaded directly on the host.

**`spec.hosts[*].useExistingK0s`** <boolean> (optional) — When true, k0sctl reuses the k0s binary that already exists on the host without
downloading or uploading anything. Upgrades for this host are skipped.
Cannot be combined with uploadBinary, k0sBinaryPath, or k0sDownloadURL.

**`spec.hosts[*].k0sBinaryPath`** <string> (optional) — Path to a local k0s binary to upload to the host. Useful for testing a custom
or development build of k0s without publishing a release.

**`spec.hosts[*].k0sInstallPath`** <string> (optional) — Path on the host where the k0s binary will be installed.

**`spec.hosts[*].k0sDownloadURL`** <string> (optional) — URL to download the k0s binary from instead of the default k0s GitHub releases.
Supports %-prefixed tokens: %v (version), %p (arch), %x (.exe on Windows).

**`spec.hosts[*].installFlags`** <string[]> (optional) — Extra flags passed verbatim to the k0s install command on the host.
See k0s install --help for available options.

**`spec.hosts[*].files`** <object[]> (optional) — Files to upload to the host before k0s is configured. Supports local paths,
URLs, and glob patterns. See the file upload documentation for details.

**`spec.hosts[*].os`** <string> (optional) — Override OS distribution auto-detection. By default k0sctl reads /etc/os-release.
Set this when the release file does not reflect the true distribution, e.g. set
"debian" for a Debian-based image that reports a different OS ID.

**`spec.hosts[*].hostname`** <string> (optional) — Override the hostname reported by the OS. When not set, the OS hostname is used.

**`spec.hosts[*].noTaints`** <boolean> (optional) — When true and used with the controller+worker role, disables the default
node-role.kubernetes.io/master:NoSchedule taint so that regular workloads
can be scheduled on the node without requiring a toleration.

**`spec.hosts[*].hooks`** <object> (optional) — Commands to run on the host at specific points during k0sctl operations.
See the hooks documentation for available stages and timing details.

#### `spec.hosts[*].winRM`

**`spec.hosts[*].winRM.address`** <string> (required) — IP address or hostname of the host.

**`spec.hosts[*].winRM.user`** <string> (optional) (default: `Administrator`) — WinRM user name. The user must have administrative privileges.

**`spec.hosts[*].winRM.port`** <integer> (optional) (default: `5985`) — TCP port for the WinRM endpoint. When useHTTPS is true, the default port automatically switches to 5986.

**`spec.hosts[*].winRM.password`** <string> (optional) — Password for the WinRM user. Required unless certificate-based authentication is configured.

**`spec.hosts[*].winRM.useHTTPS`** <boolean> (optional) (default: `false`) — Enable HTTPS for WinRM. When enabled, set caCertPath (and optionally certPath/keyPath) to verify the remote endpoint.

**`spec.hosts[*].winRM.insecure`** <boolean> (optional) (default: `false`) — Skip TLS certificate verification when connecting over HTTPS.

**`spec.hosts[*].winRM.useNTLM`** <boolean> (optional) (default: `false`) — Use NTLM authentication instead of basic authentication.

**`spec.hosts[*].winRM.caCertPath`** <string> (optional) — Path to a CA bundle used to validate the WinRM server certificate.

**`spec.hosts[*].winRM.certPath`** <string> (optional) — Client certificate for mutual TLS authentication.

**`spec.hosts[*].winRM.keyPath`** <string> (optional) — Private key that matches certPath.

**`spec.hosts[*].winRM.tlsServerName`** <string> (optional) — Override the TLS server name used during certificate verification.

**`spec.hosts[*].winRM.bastion`** <object> (optional) — SSH bastion (jump host) configuration.

#### `spec.hosts[*].winRM.bastion`

**`spec.hosts[*].winRM.bastion.address`** <string> (required) — IP address or hostname of the bastion host.

**`spec.hosts[*].winRM.bastion.user`** <string> (optional) (default: `root`) — SSH user for the bastion host.

**`spec.hosts[*].winRM.bastion.port`** <integer> (optional) (default: `22`) — SSH port on the bastion host.

**`spec.hosts[*].winRM.bastion.keyPath`** <string> (optional) — Path to an SSH private key for the bastion host.

#### `spec.hosts[*].ssh`

**`spec.hosts[*].ssh.address`** <string> (required) — IP address or hostname of the host.

**`spec.hosts[*].ssh.user`** <string> (optional) (default: `root`) — Username to log in as.

**`spec.hosts[*].ssh.port`** <integer> (optional) (default: `22`) — TCP port of the SSH service on the host.

**`spec.hosts[*].ssh.keyPath`** <string> (optional) — Path to an SSH private key file. If a public key is used, ssh-agent is required. When left empty, the default value will first be looked for from the SSH configuration IdentityFile parameter.

**`spec.hosts[*].ssh.bastion`** <object> (optional) — SSH bastion (jump host) configuration.

#### `spec.hosts[*].ssh.bastion`

**`spec.hosts[*].ssh.bastion.address`** <string> (required) — IP address or hostname of the bastion host.

**`spec.hosts[*].ssh.bastion.user`** <string> (optional) (default: `root`) — SSH user for the bastion host.

**`spec.hosts[*].ssh.bastion.port`** <integer> (optional) (default: `22`) — SSH port on the bastion host.

**`spec.hosts[*].ssh.bastion.keyPath`** <string> (optional) — Path to an SSH private key for the bastion host.

#### `spec.hosts[*].localhost`

**`spec.hosts[*].localhost.enabled`** <boolean> (optional) (default: `true`) — Must be set to true to enable the localhost connection.

#### `spec.hosts[*].openSSH`

**`spec.hosts[*].openSSH.address`** <string> (required) — IP address, hostname, or ssh config host alias of the host.

**`spec.hosts[*].openSSH.user`** <string> (optional) — Username to connect as.

**`spec.hosts[*].openSSH.port`** <integer> (optional) — Remote SSH port.

**`spec.hosts[*].openSSH.keyPath`** <string> (optional) — Path to an SSH private key.

**`spec.hosts[*].openSSH.configPath`** <string> (optional) — Path to ssh config. Defaults to ~/.ssh/config with fallback to /etc/ssh/ssh_config.

**`spec.hosts[*].openSSH.options`** <object> (optional) — Additional options as key/value pairs passed to the ssh client as -o flags.

**`spec.hosts[*].openSSH.disableMultiplexing`** <boolean> (optional) — Disable SSH connection multiplexing. When true, every remote command requires reconnecting to the host.

#### `spec.hosts[*].files[*]`

**`spec.hosts[*].files[*].name`** <string> (optional) — Optional label for this upload entry, used only in log output.

**`spec.hosts[*].files[*].src`** <string> (optional) — Source file path, URL, or glob pattern. Required when data is not set.
Glob patterns follow the doublestar syntax. URL sources are downloaded directly
on the target host. Supports %v, %p, %x token expansion.

**`spec.hosts[*].files[*].data`** <string> (optional) — Inline file content to write to the destination. Required when src is not set.

**`spec.hosts[*].files[*].dstDir`** <string> (optional) — Destination directory on the host. k0sctl creates the full path if it does
not exist. Defaults to the remote user's home directory.

**`spec.hosts[*].files[*].dst`** <string> (optional) — Destination filename on the host. Only valid for single-file uploads.
Defaults to the source file's basename.

**`spec.hosts[*].files[*].perm`** <any> (optional) — Permission mode for the uploaded file(s), e.g. 0644. Defaults to the local
file's permission mode.

**`spec.hosts[*].files[*].dirPerm`** <any> (optional) — Permission mode for directories created by k0sctl during upload.

**`spec.hosts[*].files[*].user`** <string> (optional) — Owner user name for the uploaded file(s) and created directories. Must already
exist on the host.

**`spec.hosts[*].files[*].group`** <string> (optional) — Owner group name for the uploaded file(s) and created directories. Must already
exist on the host.

#### `spec.hosts[*].hooks`

**`spec.hosts[*].hooks.connect`** <object> (optional)

**`spec.hosts[*].hooks.apply`** <object> (optional)

**`spec.hosts[*].hooks.upgrade`** <object> (optional)

**`spec.hosts[*].hooks.install`** <object> (optional)

**`spec.hosts[*].hooks.backup`** <object> (optional)

**`spec.hosts[*].hooks.reset`** <object> (optional)

#### `spec.hosts[*].hooks.connect`

**`spec.hosts[*].hooks.connect.before`** <string[]> (optional) — Commands to run before the action.

**`spec.hosts[*].hooks.connect.after`** <string[]> (optional) — Commands to run after the action.

#### `spec.hosts[*].hooks.apply`

**`spec.hosts[*].hooks.apply.before`** <string[]> (optional) — Commands to run before the action.

**`spec.hosts[*].hooks.apply.after`** <string[]> (optional) — Commands to run after the action.

#### `spec.hosts[*].hooks.upgrade`

**`spec.hosts[*].hooks.upgrade.before`** <string[]> (optional) — Commands to run before the action.

**`spec.hosts[*].hooks.upgrade.after`** <string[]> (optional) — Commands to run after the action.

#### `spec.hosts[*].hooks.install`

**`spec.hosts[*].hooks.install.before`** <string[]> (optional) — Commands to run before the action.

**`spec.hosts[*].hooks.install.after`** <string[]> (optional) — Commands to run after the action.

#### `spec.hosts[*].hooks.backup`

**`spec.hosts[*].hooks.backup.before`** <string[]> (optional) — Commands to run before the action.

**`spec.hosts[*].hooks.backup.after`** <string[]> (optional) — Commands to run after the action.

#### `spec.hosts[*].hooks.reset`

**`spec.hosts[*].hooks.reset.before`** <string[]> (optional) — Commands to run before the action.

**`spec.hosts[*].hooks.reset.after`** <string[]> (optional) — Commands to run after the action.

### `spec.k0s`

**`spec.k0s.version`** <string> (optional) — Version of k0s to deploy. When omitted, k0sctl selects the latest stable release
(or the version already running on the cluster if one exists).

**`spec.k0s.versionChannel`** <string> (optional) (default: `stable`) — Version channel used when auto-discovering the k0s version. Set to "latest" to
allow k0sctl to select pre-release versions. Has no effect when version is set.

**`spec.k0s.dynamicConfig`** <boolean> (optional) — Enable k0s dynamic configuration. When true, k0sctl only pushes the cluster-wide
configuration on first-time initialisation; subsequent applies leave it unchanged.
Use k0sctl config edit or k0s config edit to manage it afterwards.
This flag is also auto-enabled when any controller has --enable-dynamic-config in
installFlags or in its running k0s arguments.

**`spec.k0s.config`** <object> (optional) — Embedded k0s cluster configuration. See https://docs.k0sproject.io/stable/configuration/
for field reference. When omitted, the output of k0s config create is used.
The k0s config can also be placed as a separate YAML document in the same file,
using apiVersion: k0s.k0sproject.io/v1beta1 and kind: ClusterConfig.

### `spec.options`

**`spec.options.wait`** <object> (required) — Controls wait behavior for cluster operations.

**`spec.options.drain`** <object> (required) — Controls drain behavior for cluster operations.

**`spec.options.concurrency`** <object> (required) — Controls how many hosts are operated on at once.

**`spec.options.evictTaint`** <object> (required) — Controls whether a taint is applied to nodes before disruptive operations.

#### `spec.options.wait`

**`spec.options.wait.enabled`** <boolean> (optional) (default: `true`) — When false, k0sctl will not wait for k0s to become ready after restarting the
service. Equivalent to passing --no-wait on the command line.

#### `spec.options.drain`

**`spec.options.drain.enabled`** <boolean> (optional) (default: `true`) — When false, k0sctl skips draining nodes before disruptive operations such as
upgrade or reset. Equivalent to passing --no-drain on the command line.

**`spec.options.drain.gracePeriod`** <string> (optional) (default: `120s`) — How long to wait for pods to be evicted from the node before proceeding.

**`spec.options.drain.timeout`** <string> (optional) (default: `300s`) — How long to wait for the entire drain operation to complete before timing out.

**`spec.options.drain.force`** <boolean> (optional) (default: `true`) — Pass --force to kubectl drain, allowing pods without a replication controller
to be evicted.

**`spec.options.drain.ignoreDaemonSets`** <boolean> (optional) (default: `true`) — Pass --ignore-daemonsets to kubectl drain so that DaemonSet-managed pods are
not considered when draining.

**`spec.options.drain.deleteEmptyDirData`** <boolean> (optional) (default: `true`) — Pass --delete-emptydir-data to kubectl drain, allowing pods that use emptyDir
volumes (whose data will be lost) to be evicted.

**`spec.options.drain.podSelector`** <string> (required) — Label selector passed to kubectl drain to restrict which pods are considered.

**`spec.options.drain.skipWaitForDeleteTimeout`** <string> (optional) (default: `0s`) — If a pod's DeletionTimestamp is older than this duration, skip waiting for it.
Must be greater than 0s to take effect.

#### `spec.options.concurrency`

**`spec.options.concurrency.limit`** <integer> (optional) (default: `30`) — Maximum number of hosts to configure concurrently. Equivalent to --concurrency
on the command line. Set to 0 for unlimited.

**`spec.options.concurrency.workerDisruptionPercent`** <integer> (optional) (default: `10`) — Maximum percentage of worker nodes that may be disrupted simultaneously during
operations such as upgrade. Value must be between 0 and 100. This ensures a
minimum number of workers remain available during rolling operations.

**`spec.options.concurrency.uploads`** <integer> (optional) (default: `5`) — Maximum number of file uploads to perform concurrently. Equivalent to
--concurrent-uploads on the command line.

#### `spec.options.evictTaint`

**`spec.options.evictTaint.enabled`** <boolean> (optional) (default: `false`) — When true, k0sctl applies a taint to nodes before service-affecting operations
(upgrade, reset) to signal workloads to evacuate before the node is disrupted.
Can also be enabled at runtime with --evict-taint on the command line.

**`spec.options.evictTaint.taint`** <string> (optional) (default: `k0sctl.k0sproject.io/evict=true`) — Taint to apply when enabled is true. Must be in the format key=value.

**`spec.options.evictTaint.effect`** <string> (optional) (default: `NoExecute`) — Effect of the taint. Must be NoExecute, NoSchedule, or PreferNoSchedule.

**`spec.options.evictTaint.controllerWorkers`** <boolean> (optional) (default: `false`) — When true, the taint is also applied to controller+worker nodes. By default
only pure worker nodes are tainted.

## Host Requirements

- Linux nodes are supported for all roles.
- Windows nodes can join as `worker` hosts when reachable over SSH or WinRM. This support is
  experimental and requires k0s version >= 1.34.
- On Linux, the SSH user must either be `root` or have passwordless `sudo` (or `doas`) access.
  Windows workers must allow WinRM access for the configured user (defaults to `Administrator`).
- The host must fulfil the [k0s system requirements](https://docs.k0sproject.io/stable/system-requirements/).

## Host Connection Types

Each host entry must specify exactly one connection type: `ssh`, `openSSH`, `winRM`, or `localhost`.

### SSH

The built-in SSH client. No external tooling required. Windows worker nodes can also use SSH
when an SSH server is available on the host.

```yaml
- role: worker
  ssh:
    address: 10.0.0.2
    user: ubuntu
    port: 22
    keyPath: ~/.ssh/id_rsa
```

**Bastion (jump host):** tunnel connections through an intermediate host by adding a `bastion`
block. The bastion fields are identical to the SSH connection fields.

```yaml
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

**SSH agent / auth forwarding:** a host without a `keyPath` will use the running ssh-agent.
Pageant or openssh-agent can be used on Windows.

```yaml
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

### OpenSSH

Delegates connections to the system `ssh` binary. Inherits `~/.ssh/config`, agent forwarding,
multiplexing, and all other OpenSSH features. The `address` can be an IP, hostname, or any
host alias defined in `~/.ssh/config`.

```yaml
- role: controller
  openSSH:
    address: controller1   # alias from ~/.ssh/config
```

Example `~/.ssh/config` entry that the above would pick up automatically:

```
Host controller1
  Hostname 10.0.0.1
  Port 2222
  IdentityFile ~/.ssh/id_cluster
```

Additional `ssh -o` flags can be passed via `options`:

```yaml
openSSH:
  address: 10.0.0.2
  options:
    ForwardAgent: "yes"
    StrictHostKeyChecking: "no"
```

By default, a ControlMaster connection is opened and subsequent commands reuse it. Set
`disableMultiplexing: true` to reconnect for every remote command (slower, but useful for
debugging or hosts that reject multiplexing).

```yaml
openSSH:
  address: 10.0.0.2
  disableMultiplexing: true
```

### WinRM

Connects to Windows hosts via WinRM. Requires WinRM to be enabled on the target.
Windows support is limited to the `worker` role and requires k0s >= 1.34.

```yaml
- role: worker
  winRM:
    address: win-worker-1.internal
    user: Administrator
    password: ${WINRM_PASSWORD}
    useHTTPS: true
    insecure: false
```

The user must have administrative privileges. Windows does not provide a built-in way to
elevate privileges over WinRM, so the user must already have them.

When `useHTTPS` is `true` the default port switches from 5985 to 5986. Set `caCertPath` to
verify the server certificate, or `insecure: true` to skip verification in trusted environments.

To reach a WinRM host through an SSH bastion:

```yaml
- role: worker
  winRM:
    address: 10.0.0.20
    user: Administrator
    password: ${WINRM_PASSWORD}
    bastion:
      address: bastion.example.com
      user: ubuntu
      keyPath: ~/.ssh/id_rsa
```

### Localhost

Runs k0s directly on the machine executing k0sctl without any remote connection.

```yaml
- role: single
  localhost:
    enabled: true
```

## Commonly Used Host Fields

### OS Override

By default k0sctl detects the OS by reading `/etc/os-release`. Use `os` to override when the
release file does not reflect the true distribution (e.g. a Debian-based image with a custom
OS ID):

```yaml
- role: worker
  os: debian
  ssh:
    address: 10.0.0.2
```

### Private Interface / Address

Override which network interface or IP address k0sctl uses for intra-cluster communication
when fact gathering picks the wrong one:

```yaml
- role: worker
  privateInterface: eth1
  privateAddress: 10.0.0.5
  ssh:
    address: 10.0.0.2
```

### Install Flags

Extra flags passed verbatim to `k0s install` on each host. See `k0s install --help` for all
available options.

```yaml
- role: controller
  installFlags:
    - --debug
    - --enable-dynamic-config
  ssh:
    address: 10.0.0.1
```

### Environment Variables

Key-value pairs set in the k0s service environment on the host:

```yaml
- role: worker
  environment:
    HTTP_PROXY: http://proxy.example.com:3128
    NO_PROXY: 10.0.0.0/8
  ssh:
    address: 10.0.0.2
```

## Hooks

Hooks run shell commands on the remote host at specific points during k0sctl operations.
They execute using the same remote user as the connection; prefix commands with `sudo` if
elevated privileges are required. In dry-run mode hooks are printed but not executed.

```yaml
- role: worker
  ssh:
    address: 10.0.0.3
  hooks:
    connect:
      after:
        - echo "connected to $(hostname)" >> /tmp/k0sctl.log
    apply:
      before:
        - apt-get install -y nfs-common
      after:
        - echo "apply done on $(hostname)" >> /tmp/k0sctl.log
    upgrade:
      before:
        - echo "upgrading $(hostname)" >> /tmp/k0sctl.log
      after:
        - echo "upgrade done" >> /tmp/k0sctl.log
    reset:
      before:
        - echo "resetting $(hostname)" >> /tmp/k0sctl.log
```

Available hook stages and when they fire:

| Stage | Point |
|-------|-------|
| `connect.after` | Immediately after OS detection completes |
| `apply.before` | After validation, right before configuring k0s on the host |
| `apply.after` | Before disconnecting after a successful apply |
| `upgrade.before` | Before the upgrade begins on this host |
| `upgrade.after` | After the upgrade completes on this host |
| `install.before` | Just before installing k0s components (controller init, join, worker) |
| `install.after` | Immediately after k0s components are installed and ready |
| `backup.before` | Before running `k0s backup` |
| `backup.after` | Before disconnecting after a successful backup |
| `reset.before` | After gathering cluster info, right before removing k0s |
| `reset.after` | Before disconnecting after a successful reset |

## Uploading Files

The `files` list uploads local files or directories to the host before k0s is configured.
`src` supports file paths, URLs, and glob patterns. `%p`, `%v`, `%x` tokens (see Tokens)
are expanded in `src` and `k0sDownloadURL`.

```yaml
- role: controller
  ssh:
    address: 10.0.0.1
  files:
  - name: image-bundle        # optional label used in log output
    src: airgap-images.tgz
    dstDir: /var/lib/k0s/images/
    perm: 0600
  - name: manifests
    src: ./manifests/*.yaml
    dstDir: /var/lib/k0s/manifests/myapp
    perm: 0644
  - name: motd
    data: |
      Powered by k0s
    dst: /etc/motd
    perm: 0644
```

Field summary:

| Field | Description |
|-------|-------------|
| `name` | Label for log output (optional) |
| `src` | Local path, URL, or glob — required when `data` is not set |
| `data` | Inline file content — required when `src` is not set |
| `dstDir` | Destination directory; created if it does not exist (default: user home) |
| `dst` | Destination filename; only valid for single-file uploads (default: source basename) |
| `perm` | File permission mode (default: same as local file) |
| `dirPerm` | Permission mode for created directories (default: 0755) |
| `user` | Owner user name — must already exist on the host |
| `group` | Owner group name — must already exist on the host |

## k0s Configuration

### Version Auto-discovery

When `spec.k0s.version` is omitted, k0sctl queries the k0s GitHub releases API and selects
the latest stable release (or the version already running on the cluster). Set
`spec.k0s.versionChannel: latest` to include pre-releases in the search.

### Dynamic Config

When `spec.k0s.dynamicConfig` is enabled (or auto-detected because any controller has
`--enable-dynamic-config` in `installFlags` or in its running arguments), k0sctl only
pushes the cluster-wide configuration during **first-time initialisation**. Subsequent
applies do not update it; use `k0sctl config edit` or `k0s config edit` to manage it
instead. Node-specific configuration is always updated on each apply.

See [k0s Dynamic Configuration](https://docs.k0sproject.io/stable/dynamic-configuration/).

### Separate k0s Config Document

Instead of embedding the k0s cluster configuration inside `spec.k0s.config`, you can
place it as a second YAML document in the same file (or load it via a separate `--config`
flag):

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

## Options

`spec.options` controls global behaviour for cluster operations.

```yaml
spec:
  options:
    wait:
      enabled: true
    drain:
      enabled: true
      gracePeriod: 120s
      timeout: 300s
      force: true
      ignoreDaemonSets: true
      deleteEmptyDirData: true
      skipWaitForDeleteTimeout: 0s
    evictTaint:
      enabled: false
      taint: k0sctl.k0sproject.io/evict=true
      effect: NoExecute
      controllerWorkers: false
    concurrency:
      limit: 30
      workerDisruptionPercent: 10
      uploads: 5
```

### Wait

`options.wait.enabled` (default `true`) — when `false`, k0sctl does not wait for k0s to
become ready after restarting the service. Equivalent to `--no-wait` on the command line.

### Drain

`options.drain.enabled` (default `true`) — when `false`, k0sctl skips draining nodes before
disruptive operations. Equivalent to `--no-drain` on the command line.

### EvictTaint

When `options.evictTaint.enabled` is `true`, k0sctl applies a taint to nodes before
service-affecting operations (upgrade, reset) to signal workloads to evacuate in advance.
By default only `worker` nodes are tainted; set `controllerWorkers: true` to also taint
`controller+worker` nodes.

The `--evict-taint` command-line flag can also enable this at runtime.

### Concurrency

`options.concurrency.workerDisruptionPercent` (default `10`) — the maximum percentage of
worker nodes that may be disrupted simultaneously during operations such as upgrade. Set to
`0` to allow all workers at once, or to `100` to process them one at a time (the maximum
value is treated as "unlimited" workers simultaneously, which may be confusing — use a small
value for conservative upgrades).

## Tokens

The following tokens are expanded in `k0sDownloadURL` and `files[*].src`:

| Token | Meaning |
|-------|---------|
| `%%`  | Literal `%` |
| `%p`  | Host CPU architecture (`arm`, `arm64`, `amd64`) |
| `%v`  | k0s version string (e.g. `v1.32.2+k0s.0`) |
| `%x`  | Binary extension (`.exe` on Windows, empty elsewhere) |

Any other `%X` sequence is passed through unchanged.

Example:

```yaml
- role: controller
  k0sDownloadURL: https://files.example.com/k0s-files/k0s-%v-%p%x
  # Expands to: https://files.example.com/k0s-files/k0s-v1.32.2+k0s.0-amd64
```

## Environment Variable Substitution

Bash-style variable substitution is applied to the entire configuration file before parsing.

| Expression | Meaning |
|------------|---------|
| `$VAR` / `${VAR}` | Value of `VAR` |
| `${VAR:-default}` | Value of `VAR`, or `default` if unset or empty |
| `$$VAR` | Literal `$VAR` (escapes substitution) |

See [a8m/envsubst](https://github.com/a8m/envsubst#docs) for the full expression reference.

```yaml
spec:
  hosts:
  - role: controller
    ssh:
      address: ${CONTROLLER_IP}
      user: ${SSH_USER:-root}
  - role: worker
    winRM:
      address: ${WORKER_IP}
      password: ${WINRM_PASSWORD}
```
