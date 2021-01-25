package analytics

import (
	log "github.com/sirupsen/logrus"
)

type publisher interface {
	Publish(string, map[string]interface{}) error
	Close()
}

// Client is an analytics client that implements the publisher interface
var Client publisher

// NullClient is a drop in non-functional analytics publisher
type NullClient struct{}

func (c *NullClient) Initialize() error {
	return nil
}

// Publish would send a tracking event
func (c *NullClient) Publish(event string, props map[string]interface{}) error {
	log.Tracef("analytics event %s - properties: %+v", event, props)
	return nil
}

func (c *NullClient) Close() {}

func init() {
	Client = &NullClient{}
}
