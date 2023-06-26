package analytics

type publisher interface {
	Publish(string, map[string]interface{})
	Close()
}

// Client is an analytics client that implements the publisher interface
var Client publisher

// NullClient is a drop in non-functional analytics publisher
type NullClient struct{}

// Initialize does nothing
func (c *NullClient) Initialize() error {
	return nil
}

// Publish would send a tracking event
func (c *NullClient) Publish(_ string, _ map[string]interface{}) {}

// Close the analytics connection
func (c *NullClient) Close() {}

func init() {
	Client = &NullClient{}
}
