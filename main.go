package main

import (
	"os"
	"runtime"
	"strings"

	"github.com/k0sproject/k0sctl/analytics"
	"github.com/k0sproject/k0sctl/cmd"
	log "github.com/sirupsen/logrus"
)

func handlepanic() {
	if err := recover(); err != nil {
		buf := make([]byte, 1<<16)
		ss := runtime.Stack(buf, true)
		msg := string(buf[:ss])
		var bt []string
		for _, row := range strings.Split(msg, "\n") {
			if !strings.HasPrefix(row, "\t") {
				continue
			}
			if strings.Contains(row, "main.") {
				continue
			}
			if strings.Contains(row, "panic") {
				continue
			}
			bt = append(bt, strings.TrimSpace(row))
		}

		_ = analytics.Client.Publish("panic", map[string]interface{}{"backtrace": strings.Join(bt, "\n")})
		log.Fatalf("PANIC: %v\n", err)
	}
}

func main() {
	defer handlepanic()
	err := cmd.App.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
