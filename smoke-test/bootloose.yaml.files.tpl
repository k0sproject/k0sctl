cluster:
  name: k0s
  privateKey: ./id_rsa_k0s
machines:
- count: 1
  backend: docker
  spec:
    image: $LINUX_IMAGE
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
    image: $LINUX_IMAGE
    name: worker%d
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
# The file server the URL sources are downloaded from. It is not part of the
# cluster and is only reachable from the other machines, so the k0sctl config
# refers to it by its address on the container network. It comes last so that
# the ssh port mappings of the cluster machines stay as they are.
- count: 1
  backend: docker
  spec:
    image: $LINUX_IMAGE
    name: filesrv%d
    privileged: true
    portMappings:
    - containerPort: 22
      hostPort: 9022
