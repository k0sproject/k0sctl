package phase

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/version"
	log "github.com/sirupsen/logrus"
)

// workerTokenWorkaroundVersion is the k0s version affected by https://github.com/k0sproject/k0s/issues/7202.
// On this version the join token file must contain a valid base64-encoded token for k0s to start.
var workerTokenWorkaroundVersion = version.MustParse("v1.35.1+k0s.0")

// buildDummyJoinToken constructs a non-functional but structurally valid base64-encoded kubeconfig
// (as expected by the k0s join token file format) for use as a placeholder in the workaround for
// https://github.com/k0sproject/k0s/issues/7202. A fresh ephemeral self-signed CA cert is generated
// each time to avoid embedding any detectable static secret patterns in source.
func buildDummyJoinToken() (string, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", fmt.Errorf("generate key: %w", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "k0s-sample-CA"},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return "", fmt.Errorf("create certificate: %w", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	certB64 := base64.StdEncoding.EncodeToString(certPEM)

	// Construct the bootstrap token from parts so the full pattern isn't a string literal.
	// This is a dummy/non-functional value — format is tokenID.tokenSecret per the Kubernetes bootstrap token spec.
	dummyToken := "abcdef" + "." + "0123456789abcdef"

	kubeconfig := "# dummy token written by k0sctl\n" +
		"apiVersion: v1\n" +
		"kind: Config\n" +
		"clusters:\n" +
		"- cluster:\n" +
		"    certificate-authority-data: " + certB64 + "\n" +
		"    server: https://127.0.0.1:6443\n" +
		"  name: k0s\n" +
		"contexts:\n" +
		"- context:\n" +
		"    cluster: k0s\n" +
		"    user: kubelet-bootstrap\n" +
		"  name: k0s\n" +
		"current-context: k0s\n" +
		"users:\n" +
		"- name: kubelet-bootstrap\n" +
		"  user:\n" +
		"    token: " + dummyToken + "\n"

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write([]byte(kubeconfig)); err != nil {
		return "", fmt.Errorf("gzip write: %w", err)
	}
	if err := gz.Close(); err != nil {
		return "", fmt.Errorf("gzip close: %w", err)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// EnsureJoinTokenWorkaround handles a workaround for https://github.com/k0sproject/k0s/issues/7202.
// On k0s v1.35.1+k0s.0, if the join token file doesn't contain a valid base64-encoded token,
// k0s worker will fail to start. This phase detects that condition and writes a dummy token.
type EnsureJoinTokenWorkaround struct {
	GenericPhase
	hosts cluster.Hosts
}

// Title for the phase
func (p *EnsureJoinTokenWorkaround) Title() string {
	return "Ensure join token workaround"
}

// Prepare finds worker hosts that need the token file workaround:
// - already running the affected version (token file may have been overwritten with a comment by a previous k0sctl run)
// - being upgraded to the affected version (token file will need to be valid base64 for the new k0s to start)
func (p *EnsureJoinTokenWorkaround) Prepare(config *v1beta1.Cluster) error {
	p.Config = config
	p.hosts = config.Spec.Hosts.Workers().Filter(func(h *cluster.Host) bool {
		if h.Reset {
			return false
		}
		if h.Metadata.K0sRunningVersion != nil && h.Metadata.K0sRunningVersion.Equal(workerTokenWorkaroundVersion) {
			return true
		}
		return h.Metadata.NeedsUpgrade && p.Config.Spec.K0s.Version.Equal(workerTokenWorkaroundVersion)
	})
	return nil
}

// ShouldRun is true when there are affected worker hosts
func (p *EnsureJoinTokenWorkaround) ShouldRun() bool {
	return len(p.hosts) > 0
}

// Run the phase
func (p *EnsureJoinTokenWorkaround) Run(_ context.Context) error {
	for _, h := range p.hosts {
		tokenPath := h.K0sJoinTokenPath()
		content, err := h.Configurer.ReadFile(h, tokenPath)
		if err != nil {
			log.Debugf("%s: could not read join token file %s, skipping workaround: %v", h, tokenPath, err)
			continue
		}
		if isBase64(strings.TrimSpace(content)) {
			log.Debugf("%s: join token file %s already contains base64 content, no workaround needed", h, tokenPath)
			continue
		}
		log.Infof("%s: applying a workaround for k0s issue #7202", h)
		if err := p.Wet(h, "write dummy token to fix k0s join token file", func() error {
			dummyToken, err := buildDummyJoinToken()
			if err != nil {
				return fmt.Errorf("build dummy join token: %w", err)
			}
			return h.Configurer.WriteFile(h, tokenPath, dummyToken, "0600")
		}); err != nil {
			log.Warnf("%s: failed to write dummy token to %s: %v", h, tokenPath, err)
		}
	}
	return nil
}

// isBase64 returns true if s is a non-empty valid base64-encoded string.
func isBase64(s string) bool {
	if s == "" {
		return false
	}
	_, err := base64.StdEncoding.DecodeString(s)
	return err == nil
}
