package cluster

import (
	"fmt"
	"strings"
	"time"

	"github.com/creasty/defaults"
	"github.com/jellydator/validation"
)

// Options for cluster operations.
type Options struct {
	Wait        WaitOption        `yaml:"wait"`
	Drain       DrainOption       `yaml:"drain"`
	Concurrency ConcurrencyOption `yaml:"concurrency"`
	EvictTaint  EvictTaintOption  `yaml:"evictTaint"`
}

// WaitOption controls the wait behavior for cluster operations.
type WaitOption struct {
	Enabled bool `yaml:"enabled" default:"true"`
}

// DrainOption controls the drain behavior for cluster operations.
type DrainOption struct {
	Enabled     bool          `yaml:"enabled" default:"true"`
	GracePeriod time.Duration `yaml:"gracePeriod" default:"120s"`
	Timeout     time.Duration `yaml:"timeout" default:"300s"`
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

// EvictTaintOption controls whether and how a taint is applied to nodes
// before service-affecting operations like upgrade or reset.
type EvictTaintOption struct {
	Enabled           bool   `yaml:"enabled" default:"false"`
	Taint             string `yaml:"taint" default:"k0sctl.k0sproject.io/evict=true"`
	Effect            string `yaml:"effect" default:"NoExecute"`
	ControllerWorkers bool   `yaml:"controllerWorkers" default:"false"`
}

// String returns a string representation of the EvictTaintOption (<taint>:<effect>)
func (e *EvictTaintOption) String() string {
	if e == nil || !e.Enabled {
		return ""
	}
	return e.Taint + ":" + e.Effect
}

// UnmarshalYAML implements the yaml.Unmarshaler interface for EvictTaintOption.
func (e *EvictTaintOption) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type evictTaintOption EvictTaintOption
	var tmp evictTaintOption

	if err := unmarshal(&tmp); err != nil {
		return err
	}

	if err := defaults.Set(&tmp); err != nil {
		return fmt.Errorf("set defaults for evictTaint: %w", err)
	}

	*e = EvictTaintOption(tmp)
	return nil
}

// Validate checks if the EvictTaintOption is valid.
func (e *EvictTaintOption) Validate() error {
	if e == nil || !e.Enabled {
		return nil
	}

	return validation.ValidateStruct(e,
		validation.Field(&e.Taint,
			validation.Required,
			validation.By(func(value interface{}) error {
				s, _ := value.(string)
				if !strings.Contains(s, "=") {
					return fmt.Errorf("must be in the form key=value")
				}
				return nil
			}),
		),
		validation.Field(&e.Effect,
			validation.Required,
			validation.In("NoExecute", "NoSchedule", "PreferNoSchedule"),
		),
	)
}
