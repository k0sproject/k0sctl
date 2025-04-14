package cluster

import (
	"fmt"

	"github.com/creasty/defaults"
)

// Options for cluster operations.
type Options struct {
	Wait        WaitOption        `yaml:"wait,omitempty"`
	Drain       DrainOption       `yaml:"drain,omitempty"`
	Concurrency ConcurrencyOption `yaml:"concurrency,omitempty"`
}

// WaitOption controls the wait behavior for cluster operations.
type WaitOption struct {
	Enabled bool `yaml:"enabled" default:"true"`
}

// DrainOption controls the drain behavior for cluster operations.
type DrainOption struct {
	Enabled bool `yaml:"enabled" default:"true"`
}

// ConcurrencyOption controls how many hosts are operated on at once.
type ConcurrencyOption struct {
	Limit   int `yaml:"limit" default:"30"`  // Max number of hosts to operate on at once
	Uploads int `yaml:"uploads" default:"5"` // Max concurrent file uploads
}

// UnmarshalYAML implements the yaml.Unmarshaler interface for ConcurrencyOption.
func (c *ConcurrencyOption) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type concurrencyOption ConcurrencyOption
	var tmp concurrencyOption

	if err := unmarshal(&tmp); err != nil {
		return err
	}

	if err := defaults.Set(&tmp); err != nil {
		return fmt.Errorf("failed to set defaults for concurrency option: %w", err)
	}

	*c = ConcurrencyOption(tmp)
	return nil
}
