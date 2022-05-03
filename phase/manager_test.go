package phase

import (
	"fmt"
	"testing"

	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/stretchr/testify/require"
)

type conditionalPhase struct {
	shouldrunCalled bool
	runCalled       bool
}

func (p *conditionalPhase) Title() string {
	return "conditional phase"
}

func (p *conditionalPhase) ShouldRun() bool {
	p.shouldrunCalled = true
	return false
}

func (p *conditionalPhase) Run() error {
	p.runCalled = true
	return nil
}

func TestConditionalPhase(t *testing.T) {
	m := Manager{Config: &v1beta1.Cluster{Spec: &cluster.Spec{}}}
	p := &conditionalPhase{}
	m.AddPhase(p)
	require.True(t, m.Run().Success())
	require.False(t, p.runCalled, "run was not called")
	require.True(t, p.shouldrunCalled, "shouldrun was not called")
}

type configPhase struct {
	receivedConfig bool
}

func (p *configPhase) Title() string {
	return "config phase"
}

func (p *configPhase) Prepare(c *v1beta1.Cluster) error {
	p.receivedConfig = c != nil
	return nil
}

func (p *configPhase) Run() error {
	return nil
}

func TestConfigPhase(t *testing.T) {
	m := Manager{Config: &v1beta1.Cluster{Spec: &cluster.Spec{}}}
	p := &configPhase{}
	m.AddPhase(p)
	require.True(t, m.Run().Success())
	require.True(t, p.receivedConfig, "config was not received")
}

type hookedPhase struct {
	beforeCalled bool
	afterCalled  bool
	err          error
}

func (p *hookedPhase) Title() string {
	return "hooked phase"
}

func (p *hookedPhase) Before(_ string) error {
	p.beforeCalled = true
	return nil
}

func (p *hookedPhase) After(err error) error {
	p.afterCalled = true
	p.err = err
	return nil
}

func (p *hookedPhase) Run() error {
	return fmt.Errorf("run failed")
}

func TestHookedPhase(t *testing.T) {
	m := Manager{Config: &v1beta1.Cluster{Spec: &cluster.Spec{}}}
	p := &hookedPhase{}
	m.AddPhase(p)
	require.Error(t, m.Run())
	require.True(t, p.beforeCalled, "before hook was not called")
	require.True(t, p.afterCalled, "after hook was not called")
	require.EqualError(t, p.err, "run failed")
}

func TestAddPhaseBefore(t *testing.T) {
	m := Manager{Config: &v1beta1.Cluster{Spec: &cluster.Spec{}}}
	m.AddPhase(&Connect{})
	m.AddPhase(&Disconnect{})
	require.Len(t, m.phases, 2)

	require.Error(t, m.AddPhaseBefore("Foofoo to foofoo", &DetectOS{}))

	require.NoError(t, m.AddPhaseBefore("Disconnect from hosts", &DetectOS{}))
	require.Len(t, m.phases, 3)
	require.Equal(t, m.phases[1].Title(), "Detect host operating systems")
}
