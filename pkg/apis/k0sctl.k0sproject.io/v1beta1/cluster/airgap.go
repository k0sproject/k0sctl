package cluster

import (
	"encoding/hex"
	"fmt"
	"path/filepath"

	"github.com/creasty/defaults"
	"github.com/jellydator/validation"
)

const (
	AirgapSourceAuto  = "auto"
	AirgapSourceLocal = "local"
	AirgapSourceURL   = "url"

	AirgapModeUpload         = "upload"
	AirgapModeRemoteDownload = "remoteDownload"
)

// Airgap configures native k0s airgap bundle handling.
type Airgap struct {
	Enabled bool   `yaml:"enabled,omitempty" default:"false"`
	Source  string `yaml:"source,omitempty"`
	Mode    string `yaml:"mode,omitempty"`
	Path    string `yaml:"path,omitempty"`
	URL     string `yaml:"url,omitempty"`
	SHA256  string `yaml:"sha256,omitempty"`
}

// SetDefaults sets airgap defaults when airgap handling is enabled.
func (a *Airgap) SetDefaults() {
	if a == nil || !a.Enabled {
		return
	}
	if defaults.CanUpdate(a.Source) {
		a.Source = AirgapSourceAuto
	}
	if defaults.CanUpdate(a.Mode) {
		a.Mode = AirgapModeUpload
	}
}

// Validate checks airgap configuration.
func (a *Airgap) Validate() error {
	if a == nil || !a.Enabled {
		return nil
	}
	a.SetDefaults()
	if err := validation.ValidateStruct(a,
		validation.Field(&a.Source, validation.Required, validation.In(AirgapSourceAuto, AirgapSourceLocal, AirgapSourceURL)),
		validation.Field(&a.Mode, validation.Required, validation.In(AirgapModeUpload, AirgapModeRemoteDownload)),
		validation.Field(&a.Path, validation.Required.When(a.Source == AirgapSourceLocal)),
		validation.Field(&a.URL, validation.Required.When(a.Source == AirgapSourceURL)),
		validation.Field(&a.SHA256, validation.By(validateSHA256)),
	); err != nil {
		return err
	}
	if a.Mode == AirgapModeRemoteDownload {
		return fmt.Errorf("mode %q is not supported yet", AirgapModeRemoteDownload)
	}
	return nil
}

func validateSHA256(value any) error {
	checksum, ok := value.(string)
	if !ok {
		return fmt.Errorf("not a string")
	}
	if checksum == "" {
		return nil
	}
	if len(checksum) != 64 {
		return fmt.Errorf("must be 64 hex characters")
	}
	if _, err := hex.DecodeString(checksum); err != nil {
		return fmt.Errorf("must be 64 hex characters")
	}
	return nil
}

// Resolve prepares path-based airgap configuration after unmarshalling.
func (a *Airgap) Resolve(baseDir string) {
	if a == nil || a.Path == "" || filepath.IsAbs(a.Path) || baseDir == "" {
		return
	}
	a.Path = filepath.Join(baseDir, a.Path)
}
