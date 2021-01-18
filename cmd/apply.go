package cmd

import (
	"fmt"

	"github.com/k0sproject/k0sctl/config"

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
	},
	Before: actions(initLogging, initConfig),
	Action: func(ctx *cli.Context) error {
		content := ctx.String("config")

		c := config.Cluster{}
		if err := yaml.UnmarshalStrict([]byte(content), &c); err != nil {
			return err
		}

		fmt.Println(c)

		log.Debugf("Connecting to first host")
		h := c.Spec.Hosts.First()
		err := h.Connect()
		println(err)
		return err
	},
}
