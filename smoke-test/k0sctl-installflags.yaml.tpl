apiVersion: k0sctl.k0sproject.io/v1beta1
kind: cluster
spec:
  hosts:
    - role: controller
      uploadBinary: true
      installFlags:
        - "${K0S_CONTROLLER_FLAG}"
      ssh:
        address: "127.0.0.1"
        port: 9022
        keyPath: ./id_rsa_k0s
    - role: worker
      uploadBinary: true
      installFlags:
        - "${K0S_WORKER_FLAG}"
      ssh:
        address: "127.0.0.1"
        port: 9023
        keyPath: ./id_rsa_k0s
  k0s:
    version: "${K0S_VERSION}"
    config:
      spec:
        telemetry:
          enabled: false
