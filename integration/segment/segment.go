package segment

import (
	"runtime"

	"github.com/k0sproject/k0sctl/analytics"
	"github.com/k0sproject/k0sctl/version"
	segment "github.com/segmentio/analytics-go"
	log "github.com/sirupsen/logrus"
)

// WriteKey for analytics
const WriteKey string = "oU2iC4shRUBfEboaO0FDuDIUk49Ime92"

// Verbose controls the verbosity of segment analytics client
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
	Extra: map[string]interface{}{"direct": true},
}

// Client for the Segment.io analytics service
type Client struct {
	client    segment.Client
	machineID string
}

// NewClient returns a new segment analytics client
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

// Publish enqueues the sending of a tracking event
func (c Client) Publish(event string, props map[string]interface{}) {
	log.Tracef("segment event %s - properties: %+v", event, props)
	err := c.client.Enqueue(segment.Track{
		Context:     ctx,
		AnonymousId: c.machineID,
		Event:       event,
		Properties:  props,
	})
	if err != nil {
		log.Debugf("failed to submit telemetry: %s", err)
	}
}

// Close the analytics connection
func (c Client) Close() {
	c.client.Close()
}
