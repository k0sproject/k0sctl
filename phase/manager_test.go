package phase

import (
	"context"
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

func (p *conditionalPhase) Run(_ context.Context) error {
	p.runCalled = true
	return nil
}

func TestConditionalPhase(t *testing.T) {
	m := Manager{Config: &v1beta1.Cluster{Spec: &cluster.Spec{}}}
	p := &conditionalPhase{}
	m.AddPhase(p)
	require.NoError(t, m.Run(context.Background()))
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

func (p *configPhase) Run(_ context.Context) error {
	return nil
}

func TestConfigPhase(t *testing.T) {
	m := Manager{Config: &v1beta1.Cluster{Spec: &cluster.Spec{}}}
	p := &configPhase{}
	m.AddPhase(p)
	require.NoError(t, m.Run(context.Background()))
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

func (p *hookedPhase) Run(_ context.Context) error {
	return fmt.Errorf("run failed")
}

func TestHookedPhase(t *testing.T) {
	m := Manager{Config: &v1beta1.Cluster{Spec: &cluster.Spec{}}}
	p := &hookedPhase{}
	m.AddPhase(p)
	require.Error(t, m.Run(context.Background()))
	require.True(t, p.beforeCalled, "before hook was not called")
	require.True(t, p.afterCalled, "after hook was not called")
	require.EqualError(t, p.err, "run failed")
}
