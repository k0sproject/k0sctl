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
