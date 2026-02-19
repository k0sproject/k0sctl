package main

import (
	_ "embed"
	"fmt"
	"slices"
	"strings"

	"github.com/invopop/jsonschema"
)

//go:embed preamble.md
var preambleMD string

//go:embed appendix.md
var appendixMD string

func renderMarkdown(schema *jsonschema.Schema) string {
	var b strings.Builder

	b.WriteString(preambleMD)

	// Auto-generated field reference.
	defs := schema.Definitions
	root := resolveRef(schema, defs)
	if root.Properties != nil {
		renderProperties(&b, root, defs, "", root.Required)
	}

	b.WriteString(appendixMD)

	return b.String()
}

func resolveRef(s *jsonschema.Schema, defs jsonschema.Definitions) *jsonschema.Schema {
	if s == nil {
		return nil
	}
	if s.Ref != "" {
		name := strings.TrimPrefix(s.Ref, "#/$defs/")
		if d, ok := defs[name]; ok {
			return d
		}
	}
	return s
}

func schemaType(s *jsonschema.Schema, defs jsonschema.Definitions) string {
	s = resolveRef(s, defs)
	if s == nil {
		return "any"
	}
	if s.Type != "" {
		if s.Type == "array" && s.Items != nil {
			inner := resolveRef(s.Items, defs)
			if inner != nil && inner.Type != "" {
				return inner.Type + "[]"
			}
			return "array"
		}
		return s.Type
	}
	return "any"
}

func renderProperties(b *strings.Builder, schema *jsonschema.Schema, defs jsonschema.Definitions, prefix string, parentRequired []string) {
	if schema.Properties == nil {
		return
	}

	// Emit a heading for each section.
	switch prefix {
	case "":
		b.WriteString("## Configuration Fields\n\n")
	default:
		heading := strings.TrimSuffix(prefix, ".")
		level := strings.Count(heading, ".") + 2
		if level > 4 {
			level = 4
		}
		fmt.Fprintf(b, "%s `%s`\n\n", strings.Repeat("#", level), heading)
	}

	for pair := schema.Properties.Oldest(); pair != nil; pair = pair.Next() {
		name := pair.Key
		raw := pair.Value
		fieldSchema := resolveRef(raw, defs)
		fullPath := prefix + name

		required := slices.Contains(parentRequired, name)
		typ := schemaType(raw, defs)

		reqStr := "optional"
		if required && fieldSchema.Default == nil {
			reqStr = "required"
		}

		defStr := ""
		if fieldSchema.Default != nil {
			defStr = fmt.Sprintf(" (default: `%v`)", fieldSchema.Default)
		}

		desc := raw.Description
		if desc == "" {
			desc = fieldSchema.Description
		}

		fmt.Fprintf(b, "**`%s`** <%s> (%s)%s", fullPath, typ, reqStr, defStr)
		if desc != "" {
			b.WriteString(" — ")
			b.WriteString(desc)
		}
		b.WriteString("\n\n")
	}

	// Recurse into object children.
	for pair := schema.Properties.Oldest(); pair != nil; pair = pair.Next() {
		name := pair.Key
		fieldSchema := resolveRef(pair.Value, defs)
		fullPath := prefix + name

		if fieldSchema.Type == "object" && fieldSchema.Properties != nil {
			renderProperties(b, fieldSchema, defs, fullPath+".", fieldSchema.Required)
		}

		if fieldSchema.Type == "array" && fieldSchema.Items != nil {
			itemSchema := resolveRef(fieldSchema.Items, defs)
			if itemSchema != nil && itemSchema.Type == "object" && itemSchema.Properties != nil {
				renderProperties(b, itemSchema, defs, fullPath+"[*].", itemSchema.Required)
			}
		}
	}
}
