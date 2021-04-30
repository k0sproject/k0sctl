package cmd

import (
	"bufio"
	"os"
	"strconv"
	"strings"

	"github.com/creasty/defaults"
	"github.com/k0sproject/dig"
	"github.com/k0sproject/k0sctl/config"
	"github.com/k0sproject/k0sctl/config/cluster"
	"github.com/k0sproject/rig"

	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v2"
)

// DefaultK0sYaml is pretty much what "k0s default-config" outputs
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

var defaultHosts = cluster.Hosts{
	&cluster.Host{
		Connection: rig.Connection{
			SSH: &rig.SSH{
				Address: "10.0.0.1",
			},
		},
		Role: "controller",
	},
	&cluster.Host{
		Connection: rig.Connection{
			SSH: &rig.SSH{
				Address: "10.0.0.2",
			},
		},
		Role: "worker",
	},
}

func hostFromAddress(addr, role, user, keypath string) *cluster.Host {
	port := 22

	if idx := strings.Index(addr, "@"); idx > 0 {
		user = addr[:idx]
		addr = addr[idx+1:]
	}

	if idx := strings.Index(addr, ":"); idx > 0 {
		pstr := addr[idx+1:]
		if p, err := strconv.Atoi(pstr); err == nil {
			port = p
		}
		addr = addr[:idx]
	}

	host := &cluster.Host{
		Connection: rig.Connection{
			SSH: &rig.SSH{
				Address: addr,
				Port:    port,
			},
		},
	}
	if role != "" {
		host.Role = role
	} else {
		host.Role = "worker"
	}
	if user != "" {
		host.SSH.User = user
	}
	if keypath != "" {
		host.SSH.KeyPath = keypath
	}

	return host
}

var initCommand = &cli.Command{
	Name:        "init",
	Usage:       "Create a configuration template",
	Description: "Outputs a new k0sctl configuration. When a list of addresses are provided, hosts are generated into the configuration. The list of addresses can also be provided via stdin.",
	ArgsUsage:   "[[user@]address[:port] ...]",
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:  "k0s",
			Usage: "Include a skeleton k0s config section",
		},
		&cli.StringFlag{
			Name:    "cluster-name",
			Usage:   "Cluster name",
			Aliases: []string{"n"},
			Value:   "k0s-cluster",
		},
		&cli.IntFlag{
			Name:    "controller-count",
			Usage:   "The number of controllers to create when addresses are given",
			Aliases: []string{"C"},
			Value:   1,
		},
		&cli.StringFlag{
			Name:    "user",
			Usage:   "Host user when addresses given",
			Aliases: []string{"u"},
		},
		&cli.StringFlag{
			Name:    "key-path",
			Usage:   "Host key path when addresses given",
			Aliases: []string{"i"},
		},
	},
	Action: func(ctx *cli.Context) error {

		var hosts cluster.Hosts
		var addresses []string

		// Read addresses from stdin
		stat, err := os.Stdin.Stat()
		if err == nil {
			if (stat.Mode() & os.ModeCharDevice) == 0 {
				rd := bufio.NewReader(os.Stdin)
				for {
					row, _, err := rd.ReadLine()
					if err != nil {
						break
					}
					addresses = append(addresses, string(row))
				}
				if err != nil {
					return err
				}

			}
		}

		// Read addresses from args
		addresses = append(addresses, ctx.Args().Slice()...)

		role := "controller"
		for i, a := range addresses {
			a = strings.TrimSpace(a)
			if a == "" {
				continue
			}

			if i > ctx.Int("controller-count")-1 {
				role = "worker"
			}

			hosts = append(hosts, hostFromAddress(a, role, ctx.String("user"), ctx.String("key-path")))
		}

		if len(hosts) == 0 {
			hosts = defaultHosts
		}

		cfg := config.Cluster{
			APIVersion: config.APIVersion,
			Kind:       "Cluster",
			Metadata:   &config.ClusterMetadata{Name: ctx.String("cluster-name")},
			Spec: &cluster.Spec{
				Hosts: hosts,
				K0s:   cluster.K0s{},
			},
		}

		if err := defaults.Set(&cfg); err != nil {
			return err
		}

		if ctx.Bool("k0s") {
			cfg.Spec.K0s.Config = dig.Mapping{}
			if err := yaml.Unmarshal(DefaultK0sYaml, &cfg.Spec.K0s.Config); err != nil {
				return err
			}
		}

		encoder := yaml.NewEncoder(os.Stdout)
		return encoder.Encode(&cfg)
	},
}
