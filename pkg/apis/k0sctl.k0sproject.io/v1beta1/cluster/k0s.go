package cluster

import (
	"compress/gzip"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/jellydator/validation"

	"github.com/alessio/shellescape"
	"github.com/creasty/defaults"
	"github.com/k0sproject/dig"
	"github.com/k0sproject/k0sctl/pkg/retry"
	k0sctl "github.com/k0sproject/k0sctl/version"
	"github.com/k0sproject/rig/exec"
	"github.com/k0sproject/version"
	"gopkg.in/yaml.v2"
)

// K0sMinVersion is the minimum supported k0s version
const K0sMinVersion = "0.11.0-rc1"

var (
	k0sSupportedVersion   = version.MustConstraint(">= " + K0sMinVersion)
	k0sDynamicConfigSince = version.MustConstraint(">= 1.22.2+k0s.2")
)

// K0s holds configuration for bootstraping a k0s cluster
type K0s struct {
	Version       *version.Version `yaml:"version"`
	DynamicConfig bool             `yaml:"dynamicConfig"`
	Config        dig.Mapping      `yaml:"config"`
	Metadata      K0sMetadata      `yaml:"-"`
}

// K0sMetadata contains gathered information about k0s cluster
type K0sMetadata struct {
	ClusterID        string
	VersionDefaulted bool
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
func validateVersion(value interface{}) error {
	v, ok := value.(*version.Version)
	if !ok {
		return fmt.Errorf("not a version")
	}

	if v != nil && !k0sSupportedVersion.Check(v) {
		return fmt.Errorf("minimum supported k0s version is %s", k0sSupportedVersion)
	}

	return nil
}

func (k *K0s) Validate() error {
	return validation.ValidateStruct(k,
		validation.Field(&k.Version, validation.Required),
		validation.Field(&k.Version, validation.By(validateVersion)),
		validation.Field(&k.DynamicConfig, validation.By(k.validateMinDynamic())),
	)
}

func (k *K0s) validateMinDynamic() func(interface{}) error {
	return func(value interface{}) error {
		dc, ok := value.(bool)
		if !ok {
			return fmt.Errorf("not a boolean")
		}
		if !dc {
			return nil
		}

		if k.Version != nil && !k0sDynamicConfigSince.Check(k.Version) {
			return fmt.Errorf("dynamic config only available since k0s version %s", k0sDynamicConfigSince)
		}

		return nil
	}
}

// SetDefaults (implements defaults Setter interface) defaults the version to latest k0s version
func (k *K0s) SetDefaults() {
	if k.Version != nil && !k.Version.IsZero() {
		return
	}

	latest, err := version.LatestByPrerelease(k0sctl.IsPre() || k0sctl.Version == "0.0.0")
	if err == nil {
		k.Version = latest
		k.Metadata.VersionDefaulted = true
	}
}

func (k *K0s) NodeConfig() dig.Mapping {
	return dig.Mapping{
		"apiVersion": k.Config.DigString("apiVersion"),
		"kind":       k.Config.DigString("kind"),
		"Metadata": dig.Mapping{
			"name": k.Config.DigMapping("metadata")["name"],
		},
		"spec": dig.Mapping{
			"api":     k.Config.DigMapping("spec", "api"),
			"network": k.Config.DigMapping("spec", "network"),
			"storage": k.Config.DigMapping("spec", "storage"),
		},
	}
}

// GenerateToken runs the k0s token create command
func (k K0s) GenerateToken(h *Host, role string, expiry time.Duration) (string, error) {
	var k0sFlags Flags
	k0sFlags.Add(fmt.Sprintf("--role %s", role))
	k0sFlags.Add(fmt.Sprintf("--expiry %s", expiry))

	out, err := h.ExecOutput(h.Configurer.K0sCmdf("token create --help"), exec.Sudo(h))
	if err == nil && strings.Contains(out, "--config") {
		k0sFlags.Add(fmt.Sprintf("--config %s", shellescape.Quote(h.K0sConfigPath())))
	}

	k0sFlags.AddOrReplace(fmt.Sprintf("--data-dir=%s", h.K0sDataDir()))

	var token string
	err = retry.Timeout(context.TODO(), retry.DefaultTimeout, func(_ context.Context) error {
		output, err := h.ExecOutput(h.Configurer.K0sCmdf("token create %s", k0sFlags.Join()), exec.HideOutput(), exec.Sudo(h))
		if err != nil {
			return fmt.Errorf("create token: %w", err)
		}
		token = output
		return nil
	})
	return token, err
}

// GetClusterID uses kubectl to fetch the kube-system namespace uid
func (k K0s) GetClusterID(h *Host) (string, error) {
	return h.ExecOutput(h.Configurer.KubectlCmdf(h, h.K0sDataDir(), "get -n kube-system namespace kube-system -o template={{.metadata.uid}}"), exec.Sudo(h))
}

// TokenID returns a token id from a token string that can be used to invalidate the token
func TokenID(s string) (string, error) {
	b64 := make([]byte, base64.StdEncoding.DecodedLen(len(s)))
	_, err := base64.StdEncoding.Decode(b64, []byte(s))
	if err != nil {
		return "", fmt.Errorf("failed to decode token: %w", err)
	}

	sr := strings.NewReader(s)
	b64r := base64.NewDecoder(base64.StdEncoding, sr)
	gzr, err := gzip.NewReader(b64r)
	if err != nil {
		return "", fmt.Errorf("failed to create a reader for token: %w", err)
	}
	defer gzr.Close()

	c, err := io.ReadAll(gzr)
	if err != nil {
		return "", fmt.Errorf("failed to uncompress token: %w", err)
	}
	cfg := dig.Mapping{}
	err = yaml.Unmarshal(c, &cfg)
	if err != nil {
		return "", fmt.Errorf("failed to unmarshal token: %w", err)
	}

	users, ok := cfg.Dig("users").([]interface{})
	if !ok || len(users) < 1 {
		return "", fmt.Errorf("failed to find users in token")
	}

	user, ok := users[0].(dig.Mapping)
	if !ok {
		return "", fmt.Errorf("failed to find user in token")
	}

	token, ok := user.Dig("user", "token").(string)
	if !ok {
		return "", fmt.Errorf("failed to find user token in token")
	}

	idx := strings.IndexRune(token, '.')
	if idx < 0 {
		return "", fmt.Errorf("failed to find separator in token")
	}
	return token[0:idx], nil
}
