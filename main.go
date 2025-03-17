package main

import (
	"os"

	"github.com/k0sproject/k0sctl/cmd"
	log "github.com/sirupsen/logrus"

	// blank import to make sure versioninfo is included in the binary
	_ "github.com/carlmjohnson/versioninfo"
	// blank import to make sure versioninfo is included in the binary
	_ "github.com/k0sproject/k0sctl/version"
)

func main() {
	k0sctl := cmd.NewK0sctl(os.Stdin, os.Stdout, os.Stderr)
	if err := k0sctl.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
