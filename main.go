package main

import (
	"os"

	"github.com/k0sproject/k0sctl/cmd"
	log "github.com/sirupsen/logrus"
)

func main() {
	err := cmd.App.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
