package manifest

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v2"
	yamlutil "k8s.io/apimachinery/pkg/util/yaml"
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

// Reader reads Kubernetes resource definitions from input streams.
type Reader struct {
	IgnoreErrors bool
	manifests    []*ResourceDefinition
}

// ParseOption configures optional behavior for Parse.
type ParseOption func(*parseOptions)

type parseOptions struct {
	origin string
}

// WithOrigin overrides the origin name for resources parsed from the reader.
func WithOrigin(origin string) ParseOption {
	return func(po *parseOptions) {
		po.origin = origin
	}
}

// Parse parses Kubernetes resource definitions from the provided input stream. They are then available via the Resources() or GetResources(apiVersion, kind) methods.
func (r *Reader) Parse(input io.Reader, opts ...ParseOption) error {
	po := &parseOptions{}
	for _, opt := range opts {
		opt(po)
	}

	if po.origin == "" {
		if f, ok := input.(*os.File); ok {
			po.origin = f.Name()
		}
	}

	yamlReader := yamlutil.NewYAMLReader(bufio.NewReader(input))

	for {
		rawChunk, err := yamlReader.Read()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return fmt.Errorf("error reading input: %w", err)
		}

		if len(bytes.TrimSpace(rawChunk)) == 0 {
			continue
		}

		rd := &ResourceDefinition{}
		if err := yaml.Unmarshal(rawChunk, rd); err != nil {
			if r.IgnoreErrors {
				continue
			}
			return fmt.Errorf("failed to decode resource %s: %w", po.origin, err)
		}

		if rd.APIVersion == "" || rd.Kind == "" {
			if r.IgnoreErrors {
				continue
			}
			return fmt.Errorf("missing apiVersion or kind in resource %s: %w", po.origin, err)
		}

		// Store the raw chunk
		rd.Origin = po.origin
		rd.Raw = append([]byte{}, rawChunk...)
		r.manifests = append(r.manifests, rd)
	}

	return nil
}

// ParseString parses Kubernetes resource definitions from the provided string.
func (r *Reader) ParseString(input string, opts ...ParseOption) error {
	return r.Parse(strings.NewReader(input), opts...)
}

// ParseBytes parses Kubernetes resource definitions from the provided byte slice.
func (r *Reader) ParseBytes(input []byte, opts ...ParseOption) error {
	return r.Parse(bytes.NewReader(input), opts...)
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
