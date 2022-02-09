package cluster

import (
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/alessio/shellescape"
	"github.com/avast/retry-go"
	"github.com/creasty/defaults"
	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/k0sproject/dig"
	k0sctl "github.com/k0sproject/k0sctl/version"
	"github.com/k0sproject/rig/exec"
	"github.com/k0sproject/version"
	"gopkg.in/yaml.v2"
)

// K0sMinVersion is the minimum k0s version supported
const K0sMinVersion = "0.11.0-rc1"

// K0s holds configuration for bootstraping a k0s cluster
type K0s struct {
	Version  string      `yaml:"version"`
	Config   dig.Mapping `yaml:"config,omitempty"`
	Metadata K0sMetadata `yaml:"-"`
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
	vs, ok := value.(string)
	if !ok {
		return fmt.Errorf("not a string")
	}

	v, err := version.NewVersion(vs)
	if err != nil {
		return err
	}

	min, err := version.NewVersion(K0sMinVersion)
	if err != nil {
		return fmt.Errorf("internal error: k0sminversion can't be parsed: %s", err)
	}

	if v.LessThan(min) {
		return fmt.Errorf("version: minimum supported k0s version is %s", K0sMinVersion)
	}

	return nil
}

func (k *K0s) Validate() error {
	return validation.ValidateStruct(k,
		validation.Field(&k.Version, validation.Required),
		validation.Field(&k.Version, validation.By(validateVersion)),
	)
}

// SetDefaults (implements defaults Setter interface) defaults the version to latest k0s version
func (k *K0s) SetDefaults() {
	latest, err := version.LatestReleaseByPrerelease(k0sctl.IsPre() || k0sctl.Version == "0.0.0")
	if err == nil {
		k.Version = latest.String()
		k.Metadata.VersionDefaulted = true
	}

	k.Version = strings.TrimPrefix(k.Version, "v")
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

	var token string
	err = retry.Do(
		func() error {
			output, err := h.ExecOutput(h.Configurer.K0sCmdf("token create %s", k0sFlags.Join()), exec.HideOutput(), exec.Sudo(h))
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
		retry.LastErrorOnly(true),
	)
	return token, err
}

// GetClusterID uses kubectl to fetch the kube-system namespace uid
func (k K0s) GetClusterID(h *Host) (string, error) {
	return h.ExecOutput(h.Configurer.KubectlCmdf("get -n kube-system namespace kube-system -o template={{.metadata.uid}}"), exec.Sudo(h))
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
