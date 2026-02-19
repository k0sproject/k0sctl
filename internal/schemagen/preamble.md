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

