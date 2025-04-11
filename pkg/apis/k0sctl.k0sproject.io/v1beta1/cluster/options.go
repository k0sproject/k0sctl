package cluster

import (
	"fmt"
	"strings"

	"github.com/creasty/defaults"
	"github.com/jellydator/validation"
)

type Options struct {
	NoWait      ToggleOption      `yaml:"noWait,omitempty"`
	NoDrain     ToggleOption      `yaml:"noDrain,omitempty"`
	Concurrency ConcurrencyOption `yaml:"concurrency,omitempty"`
	EvictTaint  EvictTaintOption  `yaml:"evictTaint,omitempty"`
}

type ToggleOption struct {
	Enabled bool `yaml:"enabled" default:"false"`
}

// EvictTaintOption controls whether and how a taint is applied to nodes
// before service-affecting operations like upgrade or reset.
type EvictTaintOption struct {
	ToggleOption      `yaml:",inline"`
	Taint             string `yaml:"taint" default:"k0sctl.k0sproject.io/evict=true"`
	Effect            string `yaml:"effect" default:"NoExecute"`
	ControllerWorkers bool   `yaml:"controllerWorkers" default:"false"`
}

func (e *EvictTaintOption) String() string {
	if e == nil || !e.Enabled {
		return ""
	}
	return e.Taint + ":" + e.Effect
}

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

type ConcurrencyOption struct {
	Hosts   int `yaml:"hosts" default:"30"`
	Uploads int `yaml:"uploads" default:"5"`
}

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
