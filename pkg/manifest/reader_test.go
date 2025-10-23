package manifest_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/k0sproject/k0sctl/pkg/manifest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReader_ParseIgnoreErrors(t *testing.T) {
	input := `
apiVersion: v1
kind: Pod
metadata:
  name: pod1
---
invalid_yaml
---
apiVersion: v1
kind: Service
metadata:
  name: service1
`
	reader := strings.NewReader(input)
	r := &manifest.Reader{IgnoreErrors: true}

	err := r.Parse(reader)

	// Ensure no critical errors even with invalid YAML
	require.NoError(t, err, "Parse should not return an error with IgnoreErrors=true")

	// Assert that only valid manifests are parsed
	require.Equal(t, 2, r.Len(), "Expected 2 valid manifests to be parsed")

	// Validate the parsed manifests
	assert.Equal(t, "v1", r.Resources()[0].APIVersion, "Unexpected apiVersion for Pod")
	assert.Equal(t, "Pod", r.Resources()[0].Kind, "Unexpected kind for Pod")
	assert.Equal(t, "v1", r.Resources()[1].APIVersion, "Unexpected apiVersion for Service")
	assert.Equal(t, "Service", r.Resources()[1].Kind, "Unexpected kind for Service")
}

func TestReader_ParseMultipleReaders(t *testing.T) {
	input1 := `
apiVersion: v1
kind: Pod
metadata:
  name: pod1
`
	input2 := `
apiVersion: v1
kind: Service
metadata:
  name: service1
`
	r := &manifest.Reader{}

	// Parse first reader
	err := r.Parse(strings.NewReader(input1))
	require.NoError(t, err, "Parse should not return an error for input1")

	// Parse second reader
	err = r.Parse(strings.NewReader(input2))
	require.NoError(t, err, "Parse should not return an error for input2")

	// Assert that both manifests are parsed
	require.Equal(t, 2, r.Len(), "Expected 2 manifests to be parsed")

	// Validate the parsed manifests
	pod := r.Resources()[0]
	assert.Equal(t, "v1", pod.APIVersion, "Unexpected apiVersion for Pod")
	assert.Equal(t, "Pod", pod.Kind, "Unexpected kind for Pod")
	require.Len(t, pod.Raw, len(input1))

	service := r.Resources()[1]
	assert.Equal(t, "v1", service.APIVersion, "Unexpected apiVersion for Service")
	assert.Equal(t, "Service", service.Kind, "Unexpected kind for Service")
	require.Len(t, service.Raw, len(input2))
}

func TestReader_FilterResources(t *testing.T) {
	input := `
apiVersion: v1
kind: Pod
metadata:
  name: pod1
---
apiVersion: v1
kind: Service
metadata:
  name: service1
---
apiVersion: v2
kind: Pod
metadata:
  name: pod2
`
	r := &manifest.Reader{}
	require.NoError(t, r.Parse(strings.NewReader(input)))
	v1Pods := r.FilterResources(func(rd *manifest.ResourceDefinition) bool {
		return rd.APIVersion == "v1" && rd.Kind == "Pod"
	})
	v2Pods := r.FilterResources(func(rd *manifest.ResourceDefinition) bool {
		return rd.APIVersion == "v2" && rd.Kind == "Pod"
	})
	assert.Len(t, v1Pods, 1, "Expected 2 v1 Pod to be returned")
	assert.Len(t, v2Pods, 1, "Expected 1 v2 Pod to be returned")
	assert.Equal(t, "pod1", v1Pods[0].Metadata.Name, "Unexpected name for v1 Pod")
	assert.Equal(t, "pod2", v2Pods[0].Metadata.Name, "Unexpected name for v2 Pod")
	assert.NotEmpty(t, v1Pods[0].Raw, "Expected raw data to be populated")
	assert.NotEmpty(t, v2Pods[0].Raw, "Expected raw data to be populated")
}

func TestReader_GetResources(t *testing.T) {
	input := `
apiVersion: v1
kind: Pod
metadata:
  name: pod1
---
apiVersion: v1
kind: Service
metadata:
  name: service1
---
apiVersion: v1
kind: Pod
metadata:
  name: pod2
`
	reader := strings.NewReader(input)
	r := &manifest.Reader{}

	err := r.Parse(reader)
	require.NoError(t, err, "Parse should not return an error")

	// Query for Pods
	pods, err := r.GetResources("v1", "Pod")
	require.NoError(t, err, "GetResources should not return an error for Pods")
	assert.Len(t, pods, 2, "Expected 2 Pods to be returned")

	// Validate Pods
	assert.Equal(t, "Pod", pods[0].Kind, "Unexpected kind for the first Pod")
	assert.Equal(t, "Pod", pods[1].Kind, "Unexpected kind for the second Pod")

	// Query for Services
	services, err := r.GetResources("v1", "Service")
	require.NoError(t, err, "GetResources should not return an error for Services")
	assert.Len(t, services, 1, "Expected 1 Service to be returned")
}

func TestReader_ParseHandlesLargeManifest(t *testing.T) {
	largeData := strings.Repeat("x", 70*1024)
	manifestStr := fmt.Sprintf(`apiVersion: v1
kind: ConfigMap
metadata:
  name: big-config
data:
  payload: "%s"
`, largeData)

	r := &manifest.Reader{}
	require.NoError(t, r.Parse(strings.NewReader(manifestStr)))
	require.Equal(t, 1, r.Len(), "Expected a single manifest to be parsed")

	resource := r.Resources()[0]
	require.Equal(t, "ConfigMap", resource.Kind)
	require.Equal(t, "v1", resource.APIVersion)
	require.Equal(t, strings.TrimSpace(manifestStr), strings.TrimSpace(string(resource.Raw)))

	var parsed struct {
		APIVersion string            `yaml:"apiVersion"`
		Kind       string            `yaml:"kind"`
		Metadata   map[string]string `yaml:"metadata"`
		Data       map[string]string `yaml:"data"`
	}
	require.NoError(t, resource.Unmarshal(&parsed))
	require.Contains(t, parsed.Data, "payload")
	require.Len(t, parsed.Data["payload"], len(largeData))
}
