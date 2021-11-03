package cmd

import (
	"bufio"
	"os"
	"strconv"
	"strings"

	"github.com/creasty/defaults"
	"github.com/k0sproject/dig"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/rig"

	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v2"
)

// DefaultK0sYaml is pretty much what "k0s default-config" outputs
var DefaultK0sYaml = []byte(`apiVersion: k0s.k0sproject.io/v1beta1
kind: Cluster
metadata:
  name: k0s
spec:
  api:
    port: 6443
    k0sApiPort: 9443
  storage:
    type: etcd
  network:
    podCIDR: 10.244.0.0/16
    serviceCIDR: 10.96.0.0/12
    provider: kuberouter
    kuberouter:
      mtu: 0
      peerRouterIPs: ""
      peerRouterASNs: ""
      autoMTU: true
    kubeProxy:
      disabled: false
      mode: iptables
  podSecurityPolicy:
    defaultPolicy: 00-k0s-privileged
  telemetry:
    enabled: true
  installConfig:
    users:
      etcdUser: etcd
      kineUser: kube-apiserver
      konnectivityUser: konnectivity-server
      kubeAPIserverUser: kube-apiserver
      kubeSchedulerUser: kube-scheduler
  images:
    konnectivity:
      image: us.gcr.io/k8s-artifacts-prod/kas-network-proxy/proxy-agent
      version: v0.0.24
    metricsserver:
      image: gcr.io/k8s-staging-metrics-server/metrics-server
      version: v0.5.0
    kubeproxy:
      image: k8s.gcr.io/kube-proxy
      version: v1.22.1
    coredns:
      image: docker.io/coredns/coredns
      version: 1.7.0
    calico:
      cni:
        image: docker.io/calico/cni
        version: v3.18.1
      node:
        image: docker.io/calico/node
        version: v3.18.1
      kubecontrollers:
        image: docker.io/calico/kube-controllers
        version: v3.18.1
    kuberouter:
      cni:
        image: docker.io/cloudnativelabs/kube-router
        version: v1.2.1
      cniInstaller:
        image: quay.io/k0sproject/cni-node
        version: 0.1.0
    default_pull_policy: IfNotPresent
  konnectivity:
    agentPort: 8132
    adminPort: 8133
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

	_ = defaults.Set(host)

	return host
}

func buildHosts(addresses []string, ccount int, user, keypath string) cluster.Hosts {
	var hosts cluster.Hosts
	role := "controller"
	for _, a := range addresses {
		// strip trailing comments
		if idx := strings.Index(a, "#"); idx > 0 {
			a = a[:idx]
		}
		a = strings.TrimSpace(a)
		if a == "" || strings.HasPrefix(a, "#") {
			// skip empty and comment lines
			continue
		}

		if len(hosts) >= ccount {
			role = "worker"
		}

		hosts = append(hosts, hostFromAddress(a, role, user, keypath))
	}

	if len(hosts) == 0 {
		return defaultHosts
	}

	return hosts
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

		cfg := v1beta1.Cluster{
			APIVersion: v1beta1.APIVersion,
			Kind:       "Cluster",
			Metadata:   &v1beta1.ClusterMetadata{Name: ctx.String("cluster-name")},
			Spec: &cluster.Spec{
				Hosts: buildHosts(addresses, ctx.Int("controller-count"), ctx.String("user"), ctx.String("key-path")),
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
