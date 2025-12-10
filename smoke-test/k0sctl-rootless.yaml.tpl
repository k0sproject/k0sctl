apiVersion: k0sctl.k0sproject.io/v1beta1
kind: cluster
spec:
  hosts:
    - role: controller
      uploadBinary: true
      os: "$OS_OVERRIDE"
      ssh:
        address: "127.0.0.1"
        port: 9022
        # try with an absolute path
        keyPath: ${K0SCTL_SSH_KEY}
        user: ${SSH_USER}
      hooks:
        apply:
          before:
            - "echo hello > apply.hook"
          after:
            - "grep -q hello apply.hook"
    - role: worker
      uploadBinary: true
      os: "$OS_OVERRIDE"
      ssh:
        address: "127.0.0.1"
        port: 9023
        # try with a relative path
        keyPath: foo/key
        user: ${SSH_USER}
  k0s:
    version: "${K0S_VERSION}"
    config:
      spec:
        telemetry:
          enabled: false
