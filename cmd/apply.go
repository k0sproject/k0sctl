package cmd

import (
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var applyCommand = &cli.Command{
	Name:  "apply",
	Usage: "Apply a k0sctl configuration",
	Action: func(ctx *cli.Context) error {
		log.Tracef("hello from trace")
		log.Debugf("hello from debug")
		log.Infof("hello from info")

		return nil
	},
}
