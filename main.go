package main

import (
	"os"
	"regexp"

	"github.com/k0sproject/k0sctl/analytics"
	"github.com/k0sproject/k0sctl/cmd"
	log "github.com/sirupsen/logrus"
)

func cleanError(e any) string {
	if err, ok := e.(error); ok {
		ipRE := regexp.MustCompile(`\d+\.\d+\.\d+\.\d+`)
		hostRE := regexp.MustCompile(`(?:[a-zA-Z0-9]\.){1,}[a-zA-Z0-9]{2,6}`)
		userRE := regexp.MustCompile(`[a-zA-Z0-9]+@\w+`)

		res := ipRE.ReplaceAllString(err.Error(), "[...]")
		res = hostRE.ReplaceAllString(res, "[...]")
		res = userRE.ReplaceAllString(res, "[...]@")

		return res
	}

	return "unknown"
}

func handlepanic() {
	if err := recover(); err != nil {
		_ = analytics.Client.Publish("panic", map[string]interface{}{"error": cleanError(err)})
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
