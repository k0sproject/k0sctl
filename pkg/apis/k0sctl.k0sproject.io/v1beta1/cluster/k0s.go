package cluster

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/creasty/defaults"
	"github.com/jellydator/validation"
	"github.com/k0sproject/dig"
	"github.com/k0sproject/k0sctl/pkg/retry"
	"github.com/k0sproject/rig/exec"
	"github.com/k0sproject/version"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

// K0sMinVersion is the minimum supported k0s version
const K0sMinVersion = "0.11.0-rc1"

var (
	k0sSupportedVersion           = version.MustParse(K0sMinVersion)
	k0sDynamicConfigSince         = version.MustParse("1.22.2+k0s.2")
	k0sTokenCreateConfigFlagUntil = version.MustParse("v1.23.4-rc.1+k0s.0")
)

// K0s holds configuration for bootstraping a k0s cluster
type K0s struct {
	Version        *version.Version `yaml:"version,omitempty"`
	VersionChannel string           `yaml:"versionChannel,omitempty"`
	DynamicConfig  bool             `yaml:"dynamicConfig,omitempty" default:"false"`
	Config         dig.Mapping      `yaml:"config,omitempty"`
	Metadata       K0sMetadata      `yaml:"-"`
}

// K0sMetadata contains gathered information about k0s cluster
type K0sMetadata struct {
	ClusterID        string
	VersionDefaulted bool
}

// UnmarshalYAML sets in some sane defaults when unmarshaling the data from yaml
func (k *K0s) UnmarshalYAML(unmarshal func(any) error) error {
	type k0s K0s
	yk := (*k0s)(k)

	if err := unmarshal(yk); err != nil {
		return err
	}

	return defaults.Set(k)
}

// MarshalYAML implements yaml.Marshaler interface
func (k *K0s) MarshalYAML() (any, error) {
	if k == nil {
		return nil, nil
	}
	type k0s K0s
	yk := (*k0s)(k)

	yml, err := yaml.Marshal(yk)
	if err != nil {
		return nil, fmt.Errorf("marshal k0s: %w", err)
	}

	if string(yml) == "{}\n" {
		return nil, nil
	}

	return yk, nil
}

// SetDefaults sets default values
func (k *K0s) SetDefaults() {
	if k.Version == nil {
		return
	}

	if k.Version.IsZero() {
		k.Version = nil
	}
}

func validateVersion(value any) error {
	v, ok := value.(*version.Version)
	if !ok {
		return fmt.Errorf("not a version")
	}

	if v == nil || v.IsZero() {
		return nil
	}

	if v.LessThan(k0sSupportedVersion) {
		return fmt.Errorf("minimum supported k0s version is %s", k0sSupportedVersion)
	}

	return nil
}

func (k *K0s) Validate() error {
	return validation.ValidateStruct(k,
		validation.Field(&k.Version, validation.By(validateVersion)),
		validation.Field(&k.DynamicConfig, validation.By(k.validateMinDynamic())),
		validation.Field(&k.VersionChannel, validation.In("stable", "latest"), validation.When(k.VersionChannel != "")),
	)
}

func (k *K0s) validateMinDynamic() func(any) error {
	return func(value any) error {
		dc, ok := value.(bool)
		if !ok {
			return fmt.Errorf("not a boolean")
		}
		if !dc {
			return nil
		}

		if k.Version != nil && !k.Version.IsZero() && k.Version.LessThan(k0sDynamicConfigSince) {
			return fmt.Errorf("dynamic config only available since k0s version %s", k0sDynamicConfigSince)
		}

		return nil
	}
}

func (k *K0s) NodeConfig() dig.Mapping {
	return dig.Mapping{
		"apiVersion": k.Config.DigString("apiVersion"),
		"kind":       k.Config.DigString("kind"),
		"metadata": dig.Mapping{
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
func (k *K0s) GenerateToken(ctx context.Context, h *Host, role string, expiry time.Duration) (string, error) {
	var k0sFlags Flags
	k0sFlags.Add(fmt.Sprintf("--role %s", role))
	k0sFlags.Add(fmt.Sprintf("--expiry %s", expiry))

	k0sFlags.AddOrReplace(fmt.Sprintf("--data-dir=%s", quote(h.Configurer, h.Configurer.HostPath(h.K0sDataDir()))))

	if k.Version.LessThanOrEqual(k0sTokenCreateConfigFlagUntil) {
		k0sFlags.Add(fmt.Sprintf("--config %s", quote(h.Configurer, h.K0sConfigPath())))
	}

	var token string
	err := retry.WithDefaultTimeout(ctx, func(_ context.Context) error {
		output, err := h.ExecOutput(h.Configurer.K0sCmdf("token create %s", k0sFlags.Join(h.Configurer)), exec.HideOutput(), exec.Sudo(h))
		if err != nil {
			return fmt.Errorf("create token: %w", err)
		}
		token = output
		return nil
	})
	return token, err
}

// GetClusterID uses kubectl to fetch the kube-system namespace uid
func (k *K0s) GetClusterID(h *Host) (string, error) {
	return h.ExecOutput(h.Configurer.KubectlCmdf(h, h.K0sDataDir(), "get -n kube-system namespace kube-system -o template={{.metadata.uid}}"), exec.Sudo(h))
}

// TokenData is data collected from a decoded k0s token
type TokenData struct {
	ID         string
	URL        string
	Token      string
	Kubeconfig []byte
}

// WithJoinHost returns a copy of the token data with the embedded kubeconfig's
// cluster server host replaced by host (scheme and port are preserved) and the
// token re-encoded. Because an actual controller/worker join reads the server
// URL from the token file, rewriting the host here redirects the join itself,
// not just k0sctl's preflight validation.
//
// This is used to make joins target a known-reachable address (the freshly
// initialized leader) when the token-embedded address derived from
// spec.api.externalAddress is not reachable during bootstrap, e.g. an
// externally managed keepalived VIP that is not currently assigned to the
// leader.
//
// Note: the leader's serving certificate must include host in its SANs or the
// join's TLS verification will fail.
func (d TokenData) WithJoinHost(host string) (TokenData, error) {
	if len(d.Kubeconfig) == 0 {
		return d, fmt.Errorf("token has no kubeconfig data to rewrite")
	}

	cfg := dig.Mapping{}
	if err := yaml.Unmarshal(d.Kubeconfig, &cfg); err != nil {
		return d, fmt.Errorf("failed to unmarshal token: %w", err)
	}

	clusters, ok := cfg.Dig("clusters").([]any)
	if !ok || len(clusters) < 1 {
		return d, fmt.Errorf("failed to find clusters in token")
	}
	cluster, ok := clusters[0].(dig.Mapping)
	if !ok {
		return d, fmt.Errorf("failed to find cluster in token")
	}
	clusterData, ok := cluster.Dig("cluster").(dig.Mapping)
	if !ok {
		return d, fmt.Errorf("failed to find cluster data in token")
	}
	server, ok := clusterData["server"].(string)
	if !ok || server == "" {
		return d, fmt.Errorf("failed to find cluster url in token")
	}

	u, err := url.Parse(server)
	if err != nil {
		return d, fmt.Errorf("failed to parse cluster url %q: %w", server, err)
	}
	if port := u.Port(); port != "" {
		u.Host = net.JoinHostPort(host, port)
	} else {
		u.Host = host
	}
	clusterData["server"] = u.String()

	kubeconfig, err := yaml.Marshal(cfg)
	if err != nil {
		return d, fmt.Errorf("failed to marshal token: %w", err)
	}
	token, err := encodeToken(kubeconfig)
	if err != nil {
		return d, err
	}

	return TokenData{ID: d.ID, URL: u.String(), Token: token, Kubeconfig: kubeconfig}, nil
}

// encodeToken encodes a kubeconfig into a k0s join token (gzip + base64), the
// inverse of the decoding performed by ParseToken.
func encodeToken(kubeconfig []byte) (string, error) {
	var buf bytes.Buffer
	b64 := base64.NewEncoder(base64.StdEncoding, &buf)
	gz := gzip.NewWriter(b64)
	if _, err := gz.Write(kubeconfig); err != nil {
		return "", fmt.Errorf("failed to compress token: %w", err)
	}
	if err := gz.Close(); err != nil {
		return "", fmt.Errorf("failed to compress token: %w", err)
	}
	if err := b64.Close(); err != nil {
		return "", fmt.Errorf("failed to encode token: %w", err)
	}
	return buf.String(), nil
}

// ParseToken returns TokenData for a token string
func ParseToken(s string) (TokenData, error) {
	data := TokenData{Token: s}

	b64 := make([]byte, base64.StdEncoding.DecodedLen(len(s)))
	_, err := base64.StdEncoding.Decode(b64, []byte(s))
	if err != nil {
		return data, fmt.Errorf("failed to decode token: %w", err)
	}

	sr := strings.NewReader(s)
	b64r := base64.NewDecoder(base64.StdEncoding, sr)
	gzr, err := gzip.NewReader(b64r)
	if err != nil {
		return data, fmt.Errorf("failed to create a reader for token: %w", err)
	}
	defer func() {
		if err := gzr.Close(); err != nil {
			log.Warnf("failed to close token gzip reader: %v", err)
		}
	}()

	c, err := io.ReadAll(gzr)
	if err != nil {
		return data, fmt.Errorf("failed to uncompress token: %w", err)
	}
	data.Kubeconfig = c
	cfg := dig.Mapping{}
	err = yaml.Unmarshal(c, &cfg)
	if err != nil {
		return data, fmt.Errorf("failed to unmarshal token: %w", err)
	}

	users, ok := cfg.Dig("users").([]any)
	if !ok || len(users) < 1 {
		return data, fmt.Errorf("failed to find users in token")
	}

	user, ok := users[0].(dig.Mapping)
	if !ok {
		return data, fmt.Errorf("failed to find user in token")
	}

	token, ok := user.Dig("user", "token").(string)
	if !ok {
		return data, fmt.Errorf("failed to find user token in token")
	}

	idx := strings.IndexRune(token, '.')
	if idx < 0 {
		return data, fmt.Errorf("failed to find separator in token")
	}

	data.ID = token[0:idx]

	clusters, ok := cfg.Dig("clusters").([]any)
	if !ok || len(clusters) < 1 {
		return data, fmt.Errorf("failed to find clusters in token")
	}
	cluster, ok := clusters[0].(dig.Mapping)
	if !ok {
		return data, fmt.Errorf("failed to find cluster in token")
	}
	url := cluster.DigString("cluster", "server")
	if url == "" {
		return data, fmt.Errorf("failed to find cluster url in token")
	}
	data.URL = url

	return data, nil
}
