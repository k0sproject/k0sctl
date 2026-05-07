package cluster

import (
	"testing"

	"github.com/creasty/defaults"
	"github.com/k0sproject/dig"
	"github.com/k0sproject/version"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestParseToken(t *testing.T) {
	token := "H4sIAAAAAAAC/2xVXY/iOBZ9r1/BH6geO4GeAWkfKiEmGGLKjn1N/BbidAFOgjuk+Frtf18V3SPtSvN2fc/ROdaVfc9L6Q9Q9+fDqZuNLvilaj7PQ92fZy+vo9/17GU0Go3OdX+p+9loPwz+PPvjD/xn8A3/+Q19C2bfx+Pwyanqfjj8OFTlUL+Wn8P+1B+G+6sth3I2WudoWOc4FspSeYjmAqjKlaEcESWeGBpih2muRCQSNucavEEkzBWNDGoApDV1t19W6uNSbJsyRzS1mPc7TVdiDknV0qNFQmjl1zvsaZmao3RECHVd8YZEFtlEgGW8ISmXBIQiY6km+wwbr5v9yoIvVHs71pL81CAio0yYpQ2DJMFSe1InWHEZMZHQveiqa/3hf2Eg+v/FpKJdnZifHCA2aKK5IwwSsbVzYnZgJkWLdUZ8IbfCZA5CE1hSKhxliZ2rkKRxw2hxZIlSEHMgwFWCckUTi8iTmyNy+ZqJUtktO2Y9C8Wpuk8DsTUT7ehnjt9uBTQ0T7yDB9nyw+A4Tlb5wt2NbHgB5LSJpwvR2Ytpp6oKm/lG2ZvUZoDERjs9vubzamxJcZEaX6vDwLKWFeUWIoOqi7z/hWx7c2q77DfcJ5BkQQFAyxYw6xix8BZILAar8Ha3GM7l420ssZ/UZE/rrQtUytSus4ssXGKOissKkdgiOskw1fowPKRqxnFLPy0hj1pPvV6IC0t4AOhGgZDlZjFdGYdXLBVZBozKrUccW6Ra2mQNm5sF9bsHXRVqv8lB7E3XmNyZjKHTSm7Jp82HyxoJDom56HY8zgFa6/xCoOtdIL8qF8t71rDUYBZAI247ZHnpiluZn+9WNu8GsvEusFuOpvNS20J/+GUN1aN2U2kfpFQouVaBj3PsW6VgXwXVeJfSd4DlLdN2JR+gqoAed8hEBcB7OXc4J3Dl2jLuSCQCL0pHo9jhiCU2ygCcSC3hh2moFEQWNTFvfaQS2snGLJXDMdfFWCiquBKRUh8XqZZXgZIbaJEYTLbcUQnBtLDkY8VbWuzmMAhH97ka1tWWKN1lvQFLICEb3tq+0vu+VNXEPqKvN/gQjkQSsejLv3BsUjTRNk8mpNbMF46d1Ju/SURPRWihBOJtS5eVwp9ZQhvIB8+UCo1ksSXg7IPcS2wNc35cphHKVKNE4rebbSR2ODpxd5uYAA/VfH+JW9Jt1GRv231eJ9mj1uao2+Z7pRrB2ulP4+xF5kOxDtUF3PLKJXmXCb4XgQmzuRFVmmGZnCaA/nrIBdCvuRduvMpVs8lcNi7UcDVhRG0A93JLYpP66yqYgJoLoZumlQ9x2xFD8znIkux77oacdWqSdZSVyjCWnkKmb+9WDz/Nh5+b9O1SIDIUHaC6bW5V4qFsYSnSRmUIloXCuV1MaE7IsQAxBkR5ndqASRZtFDVGm7VszHGzwEfhJqzUzTV2tMi1iG369dfsmjVvkxKKfhMPgjsccEUPLMmCTcJCsTDrfGHGdXsOJcBpo4ezQd7sQroC3EQrdLtVD+Z16lZCY58rEO8SrX7vZiId/+AIckiaRa5YBIl67uU1P/3rZTTqyraejRw6v1Snbqhvw6+U+FX/Som/I+PJ+mp8np+nz13d1MPr7nQazkNf+v9X++z7uhte/1Z6Nt2hs7NRfOp+HD5efF//qPu6q+rzbPTv/7x8qT7Nf4v8g/zT+HmF4eTqbjY6fD+E949vVzeZ7vHx8mM6uPCATi//DQAA//+MVAsnAgcAAA=="

	tokendata, err := ParseToken(token)
	require.NoError(t, err)
	require.Equal(t, "i6i3yg", tokendata.ID)
	require.Equal(t, "https://172.17.0.2:6443", tokendata.URL)
}

func TestUnmarshal(t *testing.T) {
	t.Run("version given", func(t *testing.T) {
		k0s := &K0s{}
		err := yaml.Unmarshal([]byte("version: 0.11.0-rc1\ndynamicConfig: false\n"), k0s)
		require.NoError(t, err)
		require.Equal(t, "v0.11.0-rc1", k0s.Version.String())
		require.NoError(t, k0s.Validate())
	})

	t.Run("version not given", func(t *testing.T) {
		k0s := &K0s{}
		err := yaml.Unmarshal([]byte("dynamicConfig: false\n"), k0s)
		require.NoError(t, err)
		require.NoError(t, k0s.Validate())
	})
}

func TestVersionDefaulting(t *testing.T) {
	t.Run("version given", func(t *testing.T) {
		k0s := &K0s{Version: version.MustParse("v0.11.0-rc1")}
		require.NoError(t, defaults.Set(k0s))
		require.NoError(t, k0s.Validate())
	})
}

func TestAirgapDefaults(t *testing.T) {
	k0s := &K0s{Version: version.MustParse("v1.34.1+k0s.0"), Airgap: &Airgap{Enabled: true}}

	require.NoError(t, defaults.Set(k0s))
	require.NoError(t, k0s.Validate())
	require.Equal(t, AirgapSourceAuto, k0s.Airgap.Source)
	require.Equal(t, AirgapModeUpload, k0s.Airgap.Mode)
}

func TestAirgapValidateSetsDefaults(t *testing.T) {
	airgap := &Airgap{Enabled: true}

	require.NoError(t, airgap.Validate())
	require.Equal(t, AirgapSourceAuto, airgap.Source)
	require.Equal(t, AirgapModeUpload, airgap.Mode)
}

func TestAirgapValidation(t *testing.T) {
	t.Run("local requires path", func(t *testing.T) {
		k0s := &K0s{Version: version.MustParse("v1.34.1+k0s.0"), Airgap: &Airgap{Enabled: true, Source: AirgapSourceLocal}}
		require.ErrorContains(t, k0s.Validate(), "Path: cannot be blank")
	})

	t.Run("url requires url", func(t *testing.T) {
		k0s := &K0s{Version: version.MustParse("v1.34.1+k0s.0"), Airgap: &Airgap{Enabled: true, Source: AirgapSourceURL}}
		require.ErrorContains(t, k0s.Validate(), "URL: cannot be blank")
	})

	t.Run("invalid source", func(t *testing.T) {
		k0s := &K0s{Version: version.MustParse("v1.34.1+k0s.0"), Airgap: &Airgap{Enabled: true, Source: "other"}}
		require.ErrorContains(t, k0s.Validate(), "Source: must be a valid value")
	})

	t.Run("remote download deferred", func(t *testing.T) {
		k0s := &K0s{Version: version.MustParse("v1.34.1+k0s.0"), Airgap: &Airgap{Enabled: true, Mode: AirgapModeRemoteDownload}}
		require.ErrorContains(t, k0s.Validate(), `mode "remoteDownload" is not supported yet`)
	})

	t.Run("invalid sha256", func(t *testing.T) {
		k0s := &K0s{Version: version.MustParse("v1.34.1+k0s.0"), Airgap: &Airgap{Enabled: true, Source: AirgapSourceURL, URL: "https://example.invalid/bundle", SHA256: "abc123"}}
		require.ErrorContains(t, k0s.Validate(), "SHA256: must be 64 hex characters")
	})

	t.Run("auto source rejects sha256", func(t *testing.T) {
		k0s := &K0s{Version: version.MustParse("v1.34.1+k0s.0"), Airgap: &Airgap{Enabled: true, SHA256: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}}
		require.ErrorContains(t, k0s.Validate(), `SHA256: must be empty when source is "auto"`)
	})

	t.Run("airgap requires version", func(t *testing.T) {
		k0s := &K0s{Airgap: &Airgap{Enabled: true}}
		require.ErrorContains(t, k0s.Validate(), "version is required when airgap is enabled")
	})
}

func TestNodeConfigUsesLowercaseMetadataKey(t *testing.T) {
	k0s := &K0s{
		Config: dig.Mapping{
			"apiVersion": "k0s.k0sproject.io/v1beta1",
			"kind":       "ClusterConfig",
			"metadata": dig.Mapping{
				"name": "k0s",
			},
			"spec": dig.Mapping{
				"api": dig.Mapping{
					"address": "10.0.0.1",
				},
				"network": dig.Mapping{
					"provider": "kuberouter",
				},
				"storage": dig.Mapping{
					"type": "etcd",
				},
			},
		},
	}

	nodeConfig := k0s.NodeConfig()

	require.Contains(t, nodeConfig, "metadata")
	require.NotContains(t, nodeConfig, "Metadata")
	require.Equal(t, "k0s", nodeConfig.DigString("metadata", "name"))
}
