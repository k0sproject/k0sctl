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

// DefaultK0sYaml is pretty much what "k0s config create" outputs
var DefaultK0sYaml = []byte(`apiVersion: k0s.k0sproject.io/v1beta1
kind: ClusterConfig
metadata:
  name: k0s
  namespace: kube-system
spec:
  api:
    address: 192.168.5.15
    ca:
      certificatesExpireAfter: 8760h0m0s
      expiresAfter: 87600h0m0s
    k0sApiPort: 9443
    port: 6443
    sans:
    - 192.168.5.15
    - fe80::5055:55ff:febb:c314
  controllerManager: {}
  extensions:
    helm:
      concurrencyLevel: 5
  installConfig:
    users:
      etcdUser: etcd
      kineUser: kube-apiserver
      konnectivityUser: konnectivity-server
      kubeAPIserverUser: kube-apiserver
      kubeSchedulerUser: kube-scheduler
  konnectivity:
    adminPort: 8133
    agentPort: 8132
  network:
    clusterDomain: cluster.local
    dualStack:
      enabled: false
    kubeProxy:
      iptables:
        minSyncPeriod: 0s
        syncPeriod: 0s
      ipvs:
        minSyncPeriod: 0s
        syncPeriod: 0s
        tcpFinTimeout: 0s
        tcpTimeout: 0s
        udpTimeout: 0s
      metricsBindAddress: 0.0.0.0:10249
      mode: iptables
      nftables:
        minSyncPeriod: 0s
        syncPeriod: 0s
    kuberouter:
      autoMTU: true
      hairpin: Enabled
      metricsPort: 8080
    nodeLocalLoadBalancing:
      enabled: false
      envoyProxy:
        apiServerBindPort: 7443
        konnectivityServerBindPort: 7132
      type: EnvoyProxy
    podCIDR: 10.244.0.0/16
    provider: kuberouter
    serviceCIDR: 10.96.0.0/12
  scheduler: {}
  storage:
    etcd:
      ca:
        certificatesExpireAfter: 8760h0m0s
        expiresAfter: 87600h0m0s
      peerAddress: 192.168.5.15
    type: etcd
  telemetry:
    enabled: false
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

	_ = defaults.Set(host)

	if keypath == "" {
		host.SSH.KeyPath = nil
	} else {
		host.SSH.KeyPath = &keypath
	}

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
	Before:      actions(initLogging),
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
		if inF, ok := ctx.App.Reader.(*os.File); ok {
			stat, err := inF.Stat()
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
		}

		cfg := v1beta1.Cluster{}

		if err := defaults.Set(&cfg); err != nil {
			return err
		}

		cfg.Metadata.Name = ctx.String("cluster-name")

		// Read addresses from args
		addresses = append(addresses, ctx.Args().Slice()...)
		cfg.Spec.Hosts = buildHosts(addresses, ctx.Int("controller-count"), ctx.String("user"), ctx.String("key-path"))
		for _, h := range cfg.Spec.Hosts {
			_ = defaults.Set(h)
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
