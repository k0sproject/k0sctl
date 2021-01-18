package cmd

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
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
		log.Tracef("hello from trace")
		log.Debugf("hello from debug")
		log.Infof("hello from info")

		log.Infof("reading config!")
		content := ctx.String("config")
		fmt.Println(content)

		return nil
	},
}
