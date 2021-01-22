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

type NullClient struct{}

func (c *NullClient) Initialize() error {
	return nil
}

func (c *NullClient) Publish(event string, props map[string]interface{}) error {
	log.Debugf("analytics event %s - properties: %+v", event, props)
	return nil
}

func (c *NullClient) Close() {}

func init() {
	Client = &NullClient{}
}

type titled interface {
	Title() string
}

func getTitle(o interface{}) string {
	if o, ok := o.(titled); ok {
		return o.Title()
	}
	return ""
}
