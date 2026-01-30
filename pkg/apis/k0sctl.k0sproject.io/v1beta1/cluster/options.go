package cluster

import (
	"fmt"
	"strings"
	"time"

	"github.com/creasty/defaults"
	"github.com/jellydator/validation"
	"github.com/k0sproject/k0sctl/configurer"
)

// Options for cluster operations.
type Options struct {
	Wait        WaitOption        `yaml:"wait"`
	Drain       DrainOption       `yaml:"drain"`
	Concurrency ConcurrencyOption `yaml:"concurrency"`
	EvictTaint  EvictTaintOption  `yaml:"evictTaint"`
}

// UnmarshalYAML implements the yaml.Unmarshaler interface for Options.
func (o *Options) UnmarshalYAML(unmarshal func(any) error) error {
	type options Options
	var tmp options

	if err := unmarshal(&tmp); err != nil {
		return err
	}

	if err := defaults.Set(&tmp); err != nil {
		return fmt.Errorf("failed to set defaults for options: %w", err)
	}

	*o = Options(tmp)
	return nil
}

// WaitOption controls the wait behavior for cluster operations.
type WaitOption struct {
	Enabled *bool `yaml:"enabled" default:"true"`
}

// DrainOption controls the drain behavior for cluster operations.
type DrainOption struct {
	Enabled                  *bool         `yaml:"enabled" default:"true"`
	GracePeriod              time.Duration `yaml:"gracePeriod" default:"120s"`
	Timeout                  time.Duration `yaml:"timeout" default:"300s"`
	Force                    *bool         `yaml:"force" default:"true"`
	IgnoreDaemonSets         *bool         `yaml:"ignoreDaemonSets" default:"true"`
	DeleteEmptyDirData       *bool         `yaml:"deleteEmptyDirData" default:"true"`
	PodSelector              string        `yaml:"podSelector" default:""`
	SkipWaitForDeleteTimeout time.Duration `yaml:"skipWaitForDeleteTimeout" default:"0s"`
}

// EnabledValue returns the effective enabled flag, defaulting to true when unset.
func (w WaitOption) EnabledValue() bool {
	return boolPtrValue(w.Enabled, true)
}

// EnabledValue returns the effective enabled flag, defaulting to true when unset.
func (d DrainOption) EnabledValue() bool {
	return boolPtrValue(d.Enabled, true)
}

func boolPtrValue(value *bool, def bool) bool {
	if value == nil {
		return def
	}
	return *value
}

// ToKubectlArgs converts the DrainOption to kubectl arguments.
func (d *DrainOption) ToKubectlArgs(cfg configurer.Configurer) string {
	args := []string{}

	if boolPtrValue(d.Force, true) {
		args = append(args, "--force")
	}

	if d.GracePeriod > 0 {
		args = append(args, fmt.Sprintf("--grace-period=%d", int(d.GracePeriod.Seconds())))
	}

	if d.Timeout > 0 {
		args = append(args, fmt.Sprintf("--timeout=%s", d.Timeout))
	}

	if d.PodSelector != "" {
		args = append(args, fmt.Sprintf("--pod-selector=%s", quote(cfg, d.PodSelector)))
	}

	if d.SkipWaitForDeleteTimeout > 0 {
		args = append(args, fmt.Sprintf("--skip-wait-for-delete-timeout=%s", d.SkipWaitForDeleteTimeout))
	}

	if boolPtrValue(d.DeleteEmptyDirData, true) {
		args = append(args, "--delete-emptydir-data")
	}

	if boolPtrValue(d.IgnoreDaemonSets, true) {
		args = append(args, "--ignore-daemonsets")
	}

	return strings.Join(args, " ")
}

// UnmarshalYAML implements the yaml.Unmarshaler interface for DrainOption.
func (d *DrainOption) UnmarshalYAML(unmarshal func(any) error) error {
	type drainOption DrainOption
	var tmp drainOption

	if err := unmarshal(&tmp); err != nil {
		return err
	}

	if err := defaults.Set(&tmp); err != nil {
		return fmt.Errorf("failed to set defaults for drain option: %w", err)
	}

	*d = DrainOption(tmp)
	return nil
}

// ConcurrencyOption controls how many hosts are operated on at once.
type ConcurrencyOption struct {
	Limit                   int `yaml:"limit" default:"30"`                   // Max number of hosts to operate on at once
	WorkerDisruptionPercent int `yaml:"workerDisruptionPercent" default:"10"` // Max percentage of hosts to disrupt at once
	Uploads                 int `yaml:"uploads" default:"5"`                  // Max concurrent file uploads
}

// UnmarshalYAML implements the yaml.Unmarshaler interface for ConcurrencyOption.
func (c *ConcurrencyOption) UnmarshalYAML(unmarshal func(any) error) error {
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
func (e *EvictTaintOption) UnmarshalYAML(unmarshal func(any) error) error {
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
			validation.By(func(value any) error {
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
