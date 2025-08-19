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
	fn            func() error
	beforeCalled  bool
	afterCalled   bool
	cleanupCalled bool
	runCalled     bool
	err           error
}

func (p *hookedPhase) Title() string {
	return "hooked phase"
}

func (p *hookedPhase) BeforeHook() error {
	p.beforeCalled = true
	return nil
}

func (p *hookedPhase) AfterHook() error {
	p.afterCalled = true
	return nil
}

func (p *hookedPhase) CleanUp() {
	p.cleanupCalled = true
}

func (p *hookedPhase) Run(_ context.Context) error {
	p.runCalled = true
	if p.fn != nil {
		return p.fn()
	}
	return fmt.Errorf("run failed")
}

func TestHookedPhase(t *testing.T) {
	m := Manager{Config: &v1beta1.Cluster{Spec: &cluster.Spec{}}}
	p := &hookedPhase{}
	m.AddPhase(p)
	require.Error(t, m.Run(context.Background()))
	require.True(t, p.beforeCalled, "before hook was not called")
	require.True(t, p.afterCalled, "after hook was not called")
}

func TestContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	m := Manager{Config: &v1beta1.Cluster{Spec: &cluster.Spec{}}}
	p1 := &hookedPhase{fn: func() error {
		cancel()
		return nil
	}}
	p2 := &hookedPhase{}
	m.AddPhase(p1, p2)
	require.Error(t, m.Run(ctx))
	require.Contains(t, ctx.Err().Error(), "cancel")

	require.True(t, p1.beforeCalled, "1st before hook was not called")
	require.True(t, p1.afterCalled, "1st after hook was not called")
	require.True(t, p1.runCalled, "1st run was not called")
	// this should happen because the phase was completed before the context was cancelled
	require.True(t, p1.cleanupCalled, "1st cleanup was not called")

	require.False(t, p2.beforeCalled, "2nd before hook was called")
	require.False(t, p2.afterCalled, "2nd after hook was called")
	require.False(t, p2.runCalled, "2nd run was called")
	require.False(t, p2.cleanupCalled, "2nd cleanup was called")
}
