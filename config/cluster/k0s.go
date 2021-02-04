package cluster

import (
	"strings"
	"time"

	"github.com/avast/retry-go"
	"github.com/creasty/defaults"
	"github.com/k0sproject/k0sctl/integration/github"
	"github.com/k0sproject/k0sctl/version"
	"github.com/k0sproject/rig/exec"
)

// K0sMinVersion is the minimum k0s version supported
const K0sMinVersion = "0.10.0-beta2"

// K0s holds configuration for bootstraping a k0s cluster
type K0s struct {
	Version  string      `yaml:"version" validate:"required"`
	Config   Mapping     `yaml:"config"`
	Metadata K0sMetadata `yaml:"-"`
}

// K0sMetadata contains gathered information about k0s cluster
type K0sMetadata struct {
	ClusterID string
}

// UnmarshalYAML sets in some sane defaults when unmarshaling the data from yaml
func (k *K0s) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type k0s K0s
	yk := (*k0s)(k)

	if err := unmarshal(yk); err != nil {
		return err
	}

	return defaults.Set(k)
}

// SetDefaults (implements defaults Setter interface) defaults the version to latest k0s version
func (k *K0s) SetDefaults() {
	if defaults.CanUpdate(k.Version) {
		preok := version.IsPre() || version.Version == "0.0.0"
		if latest, err := github.LatestK0sVersion(preok); err == nil {
			k.Version = latest
		}
	}

	k.Version = strings.TrimPrefix(k.Version, "v")
}

// GenerateToken runs the k0s token create command
func (k K0s) GenerateToken(h *Host, role string, expiry time.Duration) (token string, err error) {
	err = retry.Do(
		func() error {
			output, err := h.ExecOutput(h.Configurer.K0sCmdf("token create --config %s --role %s --expiry %s", h.K0sConfigPath(), role, expiry.String()), exec.HideOutput())
			if err != nil {
				return err
			}
			token = output
			return nil
		},
		retry.DelayType(retry.CombineDelay(retry.FixedDelay, retry.RandomDelay)),
		retry.MaxJitter(time.Second*2),
		retry.Delay(time.Second*3),
		retry.Attempts(60),
	)
	return
}

// GetClusterID uses kubectl to fetch the kube-system namespace uid
func (k K0s) GetClusterID(h *Host) (string, error) {
	return h.ExecOutput(h.Configurer.KubectlCmdf("get -n kube-system namespace kube-system -o template={{.metadata.uid}}"))
}
