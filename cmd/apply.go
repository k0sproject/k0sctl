package cmd

import (
	"github.com/k0sproject/k0sctl/config"
	"github.com/k0sproject/k0sctl/phase"

	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v2"
)

var applyCommand = &cli.Command{
	Name:  "apply",
	Usage: "Apply a k0sctl configuration",
	Flags: []cli.Flag{
		configFlag,
		debugFlag,
		traceFlag,
	},
	Before: actions(initLogging, initConfig),
	Action: func(ctx *cli.Context) error {
		content := ctx.String("config")

		c := config.Cluster{}
		if err := yaml.UnmarshalStrict([]byte(content), &c); err != nil {
			return err
		}

		if err := c.Validate(); err != nil {
			return err
		}

		manager := phase.Manager{Config: &c}

		manager.AddPhase(
			&phase.Connect{},
			&phase.DetectOS{},
			&phase.PrepareHosts{},
			&phase.GatherFacts{},
			&phase.DownloadBinaries{},
			&phase.UploadBinaries{},
			&phase.DownloadK0s{},
			&phase.ConfigureK0s{},
			&phase.InitializeK0s{},
			&phase.InstallControllers{},
			&phase.InstallWorkers{},
			&phase.Disconnect{},
		)

		return manager.Run()
	},
}
