package cmd

import (
	"fmt"
	"time"

	"github.com/k0sproject/k0sctl/analytics"
	"github.com/k0sproject/k0sctl/config"
	"github.com/k0sproject/k0sctl/phase"
	"github.com/logrusorgru/aurora"
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
		start := time.Now()
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

		duration := int(time.Since(start).Seconds())
		mins := duration / 60
		secs := duration % 60

		var text string
		if mins == 0 {
			text = fmt.Sprintf("==> Finished in %d seconds", secs)
		} else {
			text = fmt.Sprintf("==> Finished in %d minutes and %d seconds", mins, secs)
		}

		log.Infof(aurora.Green(text).String())

		log.Infof("k0s cluster version %s is now installed", c.Spec.K0s.Version)
		log.Infof("Tip: To access the cluster you can now fetch the admin kubeconfig using:")
		log.Infof("     " + aurora.Cyan("k0sctl kubeconfig").String())

		return nil
	},
}
