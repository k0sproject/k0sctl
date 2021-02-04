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

var initCommand = &cli.Command{
	Name:  "init",
	Usage: "Create a configuration template",
	Action: func(ctx *cli.Context) error {
		cfg := config.Cluster{
			APIVersion: config.APIVersion,
			Kind:       "cluster",
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

		encoder := yaml.NewEncoder(os.Stdout)
		return encoder.Encode(&cfg)
	},
}
