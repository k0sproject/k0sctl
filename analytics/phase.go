package analytics

import (
	"sync"
	"time"
)

// Phase is a stub implementation of a phase with analytics reporting
type Phase struct {
	props     map[string]interface{}
	start     time.Time
	propmutex sync.Mutex
}

// IncProp increases a numeric data point, creating one if it didn't exist
func (p *Phase) IncProp(key string) {
	p.propmutex.Lock()
	defer p.propmutex.Unlock()

	var val uint32
	if v, ok := p.props[key].(uint32); ok {
		val = v
	}

	val++
	p.props[key] = val
}

// SetProp sets a value to a datapoint by key
func (p *Phase) SetProp(key string, value interface{}) {
	p.propmutex.Lock()
	defer p.propmutex.Unlock()

	p.props[key] = value
}

// Before prepares the analytics properties and sets the start time
func (p *Phase) Before(title string) error {
	p.props = make(map[string]interface{})
	p.props["name"] = title
	p.start = time.Now()

	return nil
}

// After enqueues the sending of analytics
func (p *Phase) After(result error) error {
	p.props["duration"] = time.Since(p.start)

	var event string
	if result == nil {
		event = "phase-success"
	} else {
		event = "phase-failure"
	}

	Client.Publish(event, p.props)

	return nil
}
