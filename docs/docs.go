// Package docs provides the embedded k0sctl configuration documentation.
package docs

import _ "embed"

// ConfigurationMD contains the configuration reference in Markdown format.
//
//go:embed configuration.md
var ConfigurationMD []byte

// SchemaJSON contains the JSON Schema for the k0sctl configuration.
//
//go:embed k0sctl-schema.json
var SchemaJSON []byte
