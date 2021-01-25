package main

import (
	"fmt"
	"os"

	"github.com/k0sproject/k0sctl/analytics"
	"github.com/k0sproject/k0sctl/cmd"
	log "github.com/sirupsen/logrus"
)

func handlepanic() {
	if err := recover(); err != nil {
		analytics.Client.Publish("panic", map[string]interface{}{"error": fmt.Sprint(err)})
		log.Fatalf("PANIC: %s", err)
	}
}

func main() {
	defer handlepanic()
	err := cmd.App.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
