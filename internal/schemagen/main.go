// Package main generates JSON Schema and Markdown documentation from the
// k0sctl configuration Go structs.
//
// Usage: go run ./internal/schemagen
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"time"

	"github.com/invopop/jsonschema"
	"github.com/k0sproject/dig"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/rig"
	"github.com/k0sproject/version"
)

func main() {
	r := &jsonschema.Reflector{
		FieldNameTag:               "yaml",
		Mapper:                     mapCustomTypes,
		RequiredFromJSONSchemaTags: true,
	}

	// base must be the module root path; AddGoComments does path.Join(base, walkPath)
	// for each directory it visits, which must equal the package's t.PkgPath().
	if err := r.AddGoComments("github.com/k0sproject/k0sctl", "./pkg/apis/k0sctl.k0sproject.io/v1beta1"); err != nil {
		fmt.Fprintf(os.Stderr, "load go comments: %v\n", err)
		os.Exit(1)
	}

	schema := r.Reflect(&v1beta1.Cluster{})
	schema.Title = "k0sctl configuration"
	schema.Description = "Configuration file for k0sctl - a bootstrapping and management tool for k0s clusters."

	injectDefaults(schema)

	if err := os.MkdirAll("docs", 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir docs: %v\n", err)
		os.Exit(1)
	}

	// Write JSON Schema.
	jsonBytes, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal schema: %v\n", err)
		os.Exit(1)
	}
	jsonBytes = append(jsonBytes, '\n')
	if err := os.WriteFile("docs/k0sctl-schema.json", jsonBytes, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write schema: %v\n", err)
		os.Exit(1)
	}

	// Write Markdown.
	md := renderMarkdown(schema)
	if err := os.WriteFile("docs/configuration.md", []byte(md), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write markdown: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Generated docs/k0sctl-schema.json and docs/configuration.md")
}

// mapCustomTypes returns manual schema definitions for types that the
// reflector cannot handle automatically (external rig types, dig.Mapping, etc.).
func mapCustomTypes(t reflect.Type) *jsonschema.Schema {
	switch t {
	case reflect.TypeOf(dig.Mapping{}):
		return &jsonschema.Schema{
			Type:        "object",
			Description: "Arbitrary key-value mapping.",
		}
	case reflect.TypeOf(version.Version{}):
		return &jsonschema.Schema{Type: "string"}
	case reflect.TypeOf(time.Duration(0)):
		return &jsonschema.Schema{
			Type:        "string",
			Description: "Duration string (e.g. \"120s\", \"5m\").",
		}
	case reflect.TypeOf(rig.SSH{}):
		return sshSchema()
	case reflect.TypeOf(rig.WinRM{}):
		return winrmSchema()
	case reflect.TypeOf(rig.OpenSSH{}):
		return openSSHSchema()
	case reflect.TypeOf(rig.Localhost{}):
		return localhostSchema()
	case reflect.TypeOf(cluster.Hooks{}):
		return hooksSchema()
	case reflect.TypeOf(cluster.Flags{}):
		return &jsonschema.Schema{
			Type:  "array",
			Items: &jsonschema.Schema{Type: "string"},
		}
	}
	return nil
}

func hooksSchema() *jsonschema.Schema {
	stageProps := jsonschema.NewProperties()
	prop(stageProps, "before", &jsonschema.Schema{
		Type:        "array",
		Items:       &jsonschema.Schema{Type: "string"},
		Description: "Commands to run before the action.",
	})
	prop(stageProps, "after", &jsonschema.Schema{
		Type:        "array",
		Items:       &jsonschema.Schema{Type: "string"},
		Description: "Commands to run after the action.",
	})
	stageSchema := &jsonschema.Schema{
		Type:       "object",
		Properties: stageProps,
	}

	props := jsonschema.NewProperties()
	for _, action := range []string{"connect", "apply", "upgrade", "install", "backup", "reset"} {
		prop(props, action, stageSchema)
	}
	return &jsonschema.Schema{
		Type:        "object",
		Description: "Hook commands to run on the host during k0sctl operations.",
		Properties:  props,
	}
}

// injectDefaults walks the schema definitions and copies `default` values from
// Go struct tags (already parsed by the reflector into Extras) into the Default
// field.
func injectDefaults(schema *jsonschema.Schema) {
	if schema.Definitions != nil {
		for _, def := range schema.Definitions {
			injectDefaultsInSchema(def)
		}
	}
	injectDefaultsInSchema(schema)
}

func injectDefaultsInSchema(s *jsonschema.Schema) {
	if s == nil {
		return
	}
	if s.Properties != nil {
		for pair := s.Properties.Oldest(); pair != nil; pair = pair.Next() {
			injectDefaultsInSchema(pair.Value)
		}
	}
	if s.Items != nil {
		injectDefaultsInSchema(s.Items)
	}
}
