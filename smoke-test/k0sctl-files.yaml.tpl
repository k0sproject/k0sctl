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
        - name: single file
          src: ./upload/toplevel.txt
          dst: /root/singlefile/renamed.txt
          user: test
          group: test
        - name: inline-data
          dst: /root/content/hello.sh
          user: test
          group: test
          perm: 0755
          data: |-
            #!/bin/sh
            echo hello
        - name: dest_dir
          src: ./upload/toplevel.txt
          dstDir: /root/destdir
        - name: perm644
          src: ./upload_chmod/script.sh
          dstDir: /root/chmod
          perm: 0644
        - name: permtransfer
          src: ./upload_chmod/script.sh
          dstDir: /root/chmod_exec
        - name: dir
          src: ./upload
          dstDir: /root/dir
        - name: glob
          src: ./upload/**/*.txt
          dstDir: /root/glob
          dirPerm: 0700
        - name: url
          src: https://api.github.com/repos/k0sproject/k0s/releases
          dst: /root/url/releases.json
        - name: url-destdir
          src: https://api.github.com/repos/k0sproject/k0s/releases
          dstDir: /root/url_destdir
    - role: worker
      uploadBinary: true
      ssh:
        address: "127.0.0.1"
        port: 9023
        keyPath: ./id_rsa_k0s
  k0s:
    version: "$K0S_VERSION"
    config:
      spec:
        telemetry:
          enabled: false
