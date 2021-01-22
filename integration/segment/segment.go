package segment

import (
	"runtime"

	"github.com/k0sproject/k0sctl/analytics"
	"github.com/k0sproject/k0sctl/version"
	segment "github.com/segmentio/analytics-go"
	log "github.com/sirupsen/logrus"
)

var WriteKey = "9uTKgGzXDVsC97cioQWpiV40GwSlXFEl"
var Verbose bool

var ctx = &segment.Context{
	App: segment.AppInfo{
		Name:      "k0sctl",
		Version:   version.Version,
		Build:     version.GitCommit,
		Namespace: "k0s",
	},
	OS: segment.OSInfo{
		Name: runtime.GOOS + " " + runtime.GOARCH,
	},
}

type Client struct {
	client    segment.Client
	machineID string
}

func NewClient() (*Client, error) {
	client, err := segment.NewWithConfig(WriteKey, segment.Config{Verbose: Verbose})
	if err != nil {
		return nil, err
	}
	id, err := analytics.MachineID()
	if err != nil {
		return nil, err
	}
	return &Client{
		client:    client,
		machineID: id,
	}, nil
}

func (c Client) Publish(event string, props map[string]interface{}) error {
	log.Debugf("segment event %s - properties: %+v", event, props)
	c.client.Enqueue(segment.Track{
		Context:     ctx,
		AnonymousId: c.machineID,
		Event:       event,
		Properties:  props,
	})
	return nil
}

func (c Client) Close() {
	c.client.Close()
}
