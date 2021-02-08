package segment

import (
	"crypto/rand"
	"fmt"
	"os"
	"runtime"

	externalip "github.com/glendc/go-external-ip"
	"github.com/k0sproject/k0sctl/analytics"
	"github.com/k0sproject/k0sctl/version"
	segment "github.com/segmentio/analytics-go"
	log "github.com/sirupsen/logrus"
)

// WriteKey for analytics
var WriteKey = ""

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
		log.Tracef("error getting machine ID: %s", err.Error())
		b := make([]byte, 8)
		_, _ = rand.Read(b)
		id = fmt.Sprintf("%x", b)
		log.Tracef("generated a random machine ID: %s", id)
	}
	log.Tracef("using %s as machine ID", id)

	consensus := externalip.DefaultConsensus(nil, nil)
	if ip, err := consensus.ExternalIP(); err == nil {
		log.Tracef("using %s as analytics ip", ip.String())
		ctx.IP = ip
	}

	return &Client{
		client:    client,
		machineID: id,
	}, nil
}

// Publish enqueues the sending of a tracking event
func (c Client) Publish(event string, props map[string]interface{}) error {
	log.Tracef("segment event %s - properties: %+v", event, props)
	return c.client.Enqueue(segment.Track{
		Context:     ctx,
		AnonymousId: c.machineID,
		Event:       event,
		Properties:  props,
	})
}

// Close the analytics connection
func (c Client) Close() {
	c.client.Close()
}

func init() {
	if WriteKey == "" {
		WriteKey = os.Getenv("SEGMENT_WRITE_KEY")
	}

}
