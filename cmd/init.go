package cmd

import (
	"os"

	"github.com/creasty/defaults"
	"github.com/k0sproject/k0sctl/config"
	"github.com/k0sproject/k0sctl/config/cluster"
	"github.com/k0sproject/rig"

	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v2"
)

var DefaultK0sYaml = []byte(`apiVersion: k0s.k0sproject.io/v1beta1
kind: Cluster
metadata:
  name: k0s-cluster
images:
  konnectivity:
    image: us.gcr.io/k8s-artifacts-prod/kas-network-proxy/proxy-agent
    version: v0.0.13
  metricsserver:
    image: gcr.io/k8s-staging-metrics-server/metrics-server
    version: v0.3.7
  kubeproxy:
    image: k8s.gcr.io/kube-proxy
    version: v1.20.2
  coredns:
    image: docker.io/coredns/coredns
    version: 1.7.0
  calico:
    cni:
      image: calico/cni
      version: v3.16.2
    flexvolume:
      image: calico/pod2daemon-flexvol
      version: v3.16.2
    node:
      image: calico/node
      version: v3.16.2
    kubecontrollers:
      image: calico/kube-controllers
      version: v3.16.2
installConfig:
  users:
    etcdUser: etcd
    kineUser: kube-apiserver
    konnectivityUser: konnectivity-server
    kubeAPIserverUser: kube-apiserver
    kubeSchedulerUser: kube-scheduler
spec:
  api:
    address: 172.17.0.2
    sans:
    - 172.17.0.2
  storage:
    type: etcd
    etcd:
      peerAddress: 172.17.0.2
  network:
    podCIDR: 10.244.0.0/16
    serviceCIDR: 10.96.0.0/12
    provider: calico
    calico:
      mode: vxlan
      vxlanPort: 4789
      vxlanVNI: 4096
      mtu: 1450
      wireguard: false
      flexVolumeDriverPath: /usr/libexec/k0s/kubelet-plugins/volume/exec/nodeagent~uds
      withWindowsNodes: false
      overlay: Always
  podSecurityPolicy:
    defaultPolicy: 00-k0s-privileged
telemetry:
  interval: 10m0s
  enabled: true
`)

var initCommand = &cli.Command{
	Name:  "init",
	Usage: "Create a configuration template",
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:  "k0s",
			Usage: "Include a skeleton k0s config section",
		},
	},
	Action: func(ctx *cli.Context) error {
		cfg := config.Cluster{
			APIVersion: config.APIVersion,
			Kind:       "Cluster",
			Metadata:   &config.ClusterMetadata{},
			Spec: &cluster.Spec{
				Hosts: cluster.Hosts{
					&cluster.Host{
						Connection: rig.Connection{
							SSH: &rig.SSH{
								Address: "10.0.0.1",
							},
						},
						Role: "server",
					},
					&cluster.Host{
						Connection: rig.Connection{
							SSH: &rig.SSH{
								Address: "10.0.0.2",
							},
						},
						Role: "worker",
					},
				},
				K0s: cluster.K0s{},
			},
		}

		if err := defaults.Set(&cfg); err != nil {
			return err
		}

		if ctx.Bool("k0s") {
			cfg.Spec.K0s.Config = cluster.Mapping{}
			if err := yaml.Unmarshal(DefaultK0sYaml, &cfg.Spec.K0s.Config); err != nil {
				return err
			}
		}

		encoder := yaml.NewEncoder(os.Stdout)
		return encoder.Encode(&cfg)
	},
}
