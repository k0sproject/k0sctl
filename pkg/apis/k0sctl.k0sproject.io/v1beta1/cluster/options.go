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
	// Controls wait behavior for cluster operations.
	Wait        WaitOption        `yaml:"wait"`
	// Controls drain behavior for cluster operations.
	Drain       DrainOption       `yaml:"drain"`
	// Controls how many hosts are operated on at once.
	Concurrency ConcurrencyOption `yaml:"concurrency"`
	// Controls whether a taint is applied to nodes before disruptive operations.
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
	// When false, k0sctl will not wait for k0s to become ready after restarting the
	// service. Equivalent to passing --no-wait on the command line.
	Enabled *bool `yaml:"enabled" default:"true" jsonschema:"default=true"`
}

// DrainOption controls the drain behavior for cluster operations.
type DrainOption struct {
	// When false, k0sctl skips draining nodes before disruptive operations such as
	// upgrade or reset. Equivalent to passing --no-drain on the command line.
	Enabled                  *bool         `yaml:"enabled" default:"true" jsonschema:"default=true"`
	// How long to wait for pods to be evicted from the node before proceeding.
	GracePeriod              time.Duration `yaml:"gracePeriod" default:"120s" jsonschema:"default=120s"`
	// How long to wait for the entire drain operation to complete before timing out.
	Timeout                  time.Duration `yaml:"timeout" default:"300s" jsonschema:"default=300s"`
	// Pass --force to kubectl drain, allowing pods without a replication controller
	// to be evicted.
	Force                    *bool         `yaml:"force" default:"true" jsonschema:"default=true"`
	// Pass --ignore-daemonsets to kubectl drain so that DaemonSet-managed pods are
	// not considered when draining.
	IgnoreDaemonSets         *bool         `yaml:"ignoreDaemonSets" default:"true" jsonschema:"default=true"`
	// Pass --delete-emptydir-data to kubectl drain, allowing pods that use emptyDir
	// volumes (whose data will be lost) to be evicted.
	DeleteEmptyDirData       *bool         `yaml:"deleteEmptyDirData" default:"true" jsonschema:"default=true"`
	// Label selector passed to kubectl drain to restrict which pods are considered.
	PodSelector              string        `yaml:"podSelector" default:""`
	// If a pod's DeletionTimestamp is older than this duration, skip waiting for it.
	// Must be greater than 0s to take effect.
	SkipWaitForDeleteTimeout time.Duration `yaml:"skipWaitForDeleteTimeout" default:"0s" jsonschema:"default=0s"`
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
	// Maximum number of hosts to configure concurrently. Equivalent to --concurrency
	// on the command line. Set to 0 for unlimited.
	Limit                   int `yaml:"limit" default:"30" jsonschema:"default=30"`
	// Maximum percentage of worker nodes that may be disrupted simultaneously during
	// operations such as upgrade. Value must be between 0 and 100. This ensures a
	// minimum number of workers remain available during rolling operations.
	WorkerDisruptionPercent int `yaml:"workerDisruptionPercent" default:"10" jsonschema:"default=10"`
	// Maximum number of file uploads to perform concurrently. Equivalent to
	// --concurrent-uploads on the command line.
	Uploads                 int `yaml:"uploads" default:"5" jsonschema:"default=5"`
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
	// When true, k0sctl applies a taint to nodes before service-affecting operations
	// (upgrade, reset) to signal workloads to evacuate before the node is disrupted.
	// Can also be enabled at runtime with --evict-taint on the command line.
	Enabled           bool   `yaml:"enabled" default:"false" jsonschema:"default=false"`
	// Taint to apply when enabled is true. Must be in the format key=value.
	Taint             string `yaml:"taint" default:"k0sctl.k0sproject.io/evict=true" jsonschema:"default=k0sctl.k0sproject.io/evict=true"`
	// Effect of the taint. Must be NoExecute, NoSchedule, or PreferNoSchedule.
	Effect            string `yaml:"effect" default:"NoExecute" jsonschema:"default=NoExecute,enum=NoExecute,enum=NoSchedule,enum=PreferNoSchedule"`
	// When true, the taint is also applied to controller+worker nodes. By default
	// only pure worker nodes are tainted.
	ControllerWorkers bool   `yaml:"controllerWorkers" default:"false" jsonschema:"default=false"`
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
