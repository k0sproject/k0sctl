package cmd

import (
	"github.com/k0sproject/k0sctl/analytics"
	"github.com/k0sproject/k0sctl/config"
	"github.com/k0sproject/k0sctl/phase"
	log "github.com/sirupsen/logrus"

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
		analyticsFlag,
	},
	Before: actions(initLogging, initConfig, displayLogo, initAnalytics, displayCopyright),
	After: func(ctx *cli.Context) error {
		analytics.Client.Close()
		return nil
	},
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
			&phase.ValidateHosts{},
			&phase.GatherK0sFacts{},
			&phase.DownloadBinaries{},
			&phase.UploadBinaries{},
			&phase.DownloadK0s{},
			&phase.ConfigureK0s{},
			&phase.InitializeK0s{},
			&phase.InstallControllers{},
			&phase.InstallWorkers{},
			&phase.Disconnect{},
		)

		if err := manager.Run(); err != nil {
			return err
		}

		log.Infof("k0s cluster version %s is now installed", c.Spec.K0s.Version)
		log.Infof("To access the cluster you can get the admin kubeconfig with:")
		log.Infof("  k0sctl kubeconfig")

		return nil
	},
}
