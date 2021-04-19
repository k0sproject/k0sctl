cluster:
  name: k0s
  privateKey: ./id_rsa_k0s
machines:
- count: 1
  backend: docker
  spec:
    image: quay.io/footloose/ubuntu18.04
    name: manager%d
    privileged: true
    volumes:
    - type: bind
      source: /lib/modules
      destination: /lib/modules
    - type: volume
      destination: /var/lib/k0s
    portMappings:
    - containerPort: 22
      hostPort: 9022
    - containerPort: 443
      hostPort: 443
    - containerPort: 6443
      hostPort: 6443
- count: 1
  backend: docker
  spec:
    image: quay.io/footloose/ubuntu18.04
    name: worker%d
    privileged: true
    volumes:
    - type: bind
      source: /lib/modules
      destination: /lib/modules
    - type: volume
      destination: /var/lib/k0s
    - type: bind
      source: $OS_RELEASE_PATH
      destination: /etc/os-release
    portMappings:
    - containerPort: 22
      hostPort: 9022