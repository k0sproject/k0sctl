# Applied on top of the k0sctl-files.yaml cluster to check that a wrong sha256
# fails the apply instead of leaving the file on the host.
apiVersion: k0sctl.k0sproject.io/v1beta1
kind: cluster
spec:
  hosts:
    - role: controller
      uploadBinary: true
      ssh:
        address: "127.0.0.1"
        port: 9022
        keyPath: ./id_rsa_k0s
      files:
        - name: badsum
          src: http://$FILESRV/files/badsum.bin
          dst: /root/srv/badsum.bin
          sha256: "0000000000000000000000000000000000000000000000000000000000000000"
  k0s:
    version: "$K0S_VERSION"
    config:
      spec:
        telemetry:
          enabled: false
