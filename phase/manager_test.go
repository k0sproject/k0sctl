package phase

import (
	"fmt"
	"testing"

	"github.com/k0sproject/k0sctl/config"
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
	m := Manager{}
	p := &conditionalPhase{}
	m.AddPhase(p)
	require.NoError(t, m.Run())
	require.False(t, p.runCalled, "run was not called")
	require.True(t, p.shouldrunCalled, "shouldrun was not called")
}

type configPhase struct {
	receivedConfig bool
}

func (p *configPhase) Title() string {
	return "config phase"
}

func (p *configPhase) Prepare(c *config.Cluster) error {
	p.receivedConfig = c != nil
	return nil
}

func (c *configPhase) Run() error {
	return nil
}

func TestConfigPhase(t *testing.T) {
	m := Manager{Config: &config.Cluster{}}
	p := &configPhase{}
	m.AddPhase(p)
	require.NoError(t, m.Run())
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

func (p *hookedPhase) Before() error {
	p.beforeCalled = true
	return nil
}

func (p *hookedPhase) After(err error) error {
	p.afterCalled = true
	p.err = err
	return nil
}

func (c *hookedPhase) Run() error {
	return fmt.Errorf("run failed")
}

func TestHookedPhase(t *testing.T) {
	m := Manager{}
	p := &hookedPhase{}
	m.AddPhase(p)
	require.Error(t, m.Run())
	require.True(t, p.beforeCalled, "before hook was not called")
	require.True(t, p.afterCalled, "after hook was not called")
	require.EqualError(t, p.err, "run failed")
}
