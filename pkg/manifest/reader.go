package manifest

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v2"
)

// ResourceDefinition represents a single Kubernetes resource definition.
type ResourceDefinition struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Origin string `yaml:"-"`
	Raw    []byte `yaml:"-"`
}

var fnRe = regexp.MustCompile(`[^\w\-\.]`)

func safeFn(input string) string {
	safe := fnRe.ReplaceAllString(input, "_")
	safe = strings.Trim(safe, "._")
	return safe
}

// Filename returns a filename compatible name of the resource definition.
func (rd *ResourceDefinition) Filename() string {
	if strings.HasSuffix(rd.Origin, ".yaml") || strings.HasSuffix(rd.Origin, ".yml") {
		return path.Base(rd.Origin)
	}

	if rd.Metadata.Name != "" {
		return fmt.Sprintf("%s-%s.yaml", safeFn(rd.Kind), safeFn(rd.Metadata.Name))
	}

	return fmt.Sprintf("%s-%s-%d.yaml", safeFn(rd.APIVersion), safeFn(rd.Kind), time.Now().UnixNano())
}

// returns a Reader that reads the raw resource definition
func (rd *ResourceDefinition) Reader() *bytes.Reader {
	return bytes.NewReader(rd.Raw)
}

// Bytes returns the raw resource definition.
func (rd *ResourceDefinition) Bytes() []byte {
	return rd.Raw
}

// Unmarshal unmarshals the raw resource definition into the provided object.
func (rd *ResourceDefinition) Unmarshal(obj any) error {
	if err := yaml.UnmarshalStrict(rd.Bytes(), obj); err != nil {
		return fmt.Errorf("failed to unmarshal %s: %w", rd.Origin, err)
	}
	return nil
}

func yamlDocumentSplit(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}

	// Look for the document separator
	sepIndex := bytes.Index(data, []byte("\n---"))
	if sepIndex >= 0 {
		// Return everything up to the separator
		return sepIndex + len("\n---"), data[:sepIndex], nil
	}

	// If at EOF, return the remaining data
	if atEOF {
		return len(data), data, nil
	}

	// Request more data
	return 0, nil, nil
}

// Reader reads Kubernetes resource definitions from input streams.
type Reader struct {
	IgnoreErrors bool
	manifests    []*ResourceDefinition
}

func name(r io.Reader) string {
	if n, ok := r.(*os.File); ok {
		return n.Name()
	}
	return "manifest"
}

// Parse parses Kubernetes resource definitions from the provided input stream. They are then available via the Resources() or GetResources(apiVersion, kind) methods.
func (r *Reader) Parse(input io.Reader) error {
	scanner := bufio.NewScanner(input)
	scanner.Split(yamlDocumentSplit)

	for scanner.Scan() {
		rawChunk := scanner.Bytes()

		// Skip empty chunks
		if len(rawChunk) == 0 {
			continue
		}

		rd := &ResourceDefinition{}
		if err := yaml.Unmarshal(rawChunk, rd); err != nil {
			if r.IgnoreErrors {
				continue
			}
			return fmt.Errorf("failed to decode resource %s: %w", name(input), err)
		}

		if rd.APIVersion == "" || rd.Kind == "" {
			if r.IgnoreErrors {
				continue
			}
			return fmt.Errorf("missing apiVersion or kind in resource %s", name(input))
		}

		// Store the raw chunk
		rd.Raw = append([]byte{}, rawChunk...)
		r.manifests = append(r.manifests, rd)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading input: %w", err)
	}

	return nil
}

// ParseString parses Kubernetes resource definitions from the provided string.
func (r *Reader) ParseString(input string) error {
	return r.Parse(strings.NewReader(input))
}

// ParseBytes parses Kubernetes resource definitions from the provided byte slice.
func (r *Reader) ParseBytes(input []byte) error {
	return r.Parse(bytes.NewReader(input))
}

// Resources returns all parsed Kubernetes resource definitions.
func (r *Reader) Resources() []*ResourceDefinition {
	return r.manifests
}

// Len returns the number of parsed Kubernetes resource definitions.
func (r *Reader) Len() int {
	return len(r.manifests)
}

// FilterResources returns all parsed Kubernetes resource definitions that match the provided filter function.
func (r *Reader) FilterResources(filter func(rd *ResourceDefinition) bool) []*ResourceDefinition {
	var resources []*ResourceDefinition
	for _, rd := range r.manifests {
		if filter(rd) {
			resources = append(resources, rd)
		}
	}
	return resources
}

// GetResources returns all parsed Kubernetes resource definitions that match the provided apiVersion and kind. The matching is case-insensitive.
func (r *Reader) GetResources(apiVersion, kind string) ([]*ResourceDefinition, error) {
	resources := r.FilterResources(func(rd *ResourceDefinition) bool {
		return strings.EqualFold(rd.APIVersion, apiVersion) && strings.EqualFold(rd.Kind, kind)
	})

	if len(resources) == 0 {
		return nil, fmt.Errorf("no resources found for apiVersion=%s, kind=%s", apiVersion, kind)
	}
	return resources, nil
}
